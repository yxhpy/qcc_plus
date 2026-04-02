package proxy

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type ErrorPolicyAction string

const (
	ErrorPolicyActionAutoSwitch ErrorPolicyAction = "auto_switch"
)

type ErrorPolicyRule struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	StatusCode      int               `json:"status_code,omitempty"`
	ErrorCode       string            `json:"error_code,omitempty"`
	MessageContains string            `json:"message_contains,omitempty"`
	Action          ErrorPolicyAction `json:"action"`
	Enabled         bool              `json:"enabled"`
	Builtin         bool              `json:"builtin"`
}

type ObservedErrorPolicy struct {
	ID         string    `json:"id"`
	StatusCode int       `json:"status_code"`
	ErrorCode  string    `json:"error_code,omitempty"`
	Message    string    `json:"message"`
	Count      int       `json:"count"`
	LastSeenAt time.Time `json:"last_seen_at"`
	AutoSwitch bool      `json:"auto_switch"`
}

type ErrorPolicySnapshot struct {
	BuiltinRules []ErrorPolicyRule     `json:"builtin_rules"`
	CustomRules  []ErrorPolicyRule     `json:"custom_rules"`
	Observed     []ObservedErrorPolicy `json:"observed"`
}

type errorPolicyObservedRecord struct {
	ID         string    `json:"id"`
	StatusCode int       `json:"status_code"`
	ErrorCode  string    `json:"error_code,omitempty"`
	Message    string    `json:"message"`
	Count      int       `json:"count"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

type ErrorPolicyManager struct {
	mu              sync.RWMutex
	cache           *SettingsCache
	customRules     map[string]ErrorPolicyRule
	disabledBuiltin map[string]bool
	observed        map[string]errorPolicyObservedRecord
}

const (
	errorPolicyCustomRulesKey     = "error_policy.custom_rules"
	errorPolicyDisabledBuiltinKey = "error_policy.disabled_builtin_ids"
	errorPolicyObservedKey        = "error_policy.observed"
	maxObservedErrorPolicies      = 300
)

func builtinErrorPolicies() []ErrorPolicyRule {
	return []ErrorPolicyRule{
		{ID: "builtin-400-packy", Name: "400 packy_api_error 自动切换", StatusCode: 400, ErrorCode: "packy_api_error", Action: ErrorPolicyActionAutoSwitch, Enabled: true, Builtin: true},
		{ID: "builtin-400-new-api", Name: "400 new_api_error 自动切换", StatusCode: 400, ErrorCode: "new_api_error", Action: ErrorPolicyActionAutoSwitch, Enabled: true, Builtin: true},
		{ID: "builtin-400-gateway", Name: "400 包含 gateway 自动切换", StatusCode: 400, MessageContains: "gateway", Action: ErrorPolicyActionAutoSwitch, Enabled: true, Builtin: true},
	}
}

func NewErrorPolicyManager(cache *SettingsCache) *ErrorPolicyManager {
	m := &ErrorPolicyManager{
		cache:           cache,
		customRules:     make(map[string]ErrorPolicyRule),
		disabledBuiltin: make(map[string]bool),
		observed:        make(map[string]errorPolicyObservedRecord),
	}
	m.reloadFromCache()
	return m
}

func (m *ErrorPolicyManager) reloadFromCache() {
	if m == nil || m.cache == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.customRules = make(map[string]ErrorPolicyRule)
	m.disabledBuiltin = make(map[string]bool)
	m.observed = make(map[string]errorPolicyObservedRecord)

	if v, ok := m.cache.Get(errorPolicyCustomRulesKey); ok {
		var rules []ErrorPolicyRule
		decodeAny(v, &rules)
		for _, r := range rules {
			if strings.TrimSpace(r.ID) == "" {
				continue
			}
			r.Enabled = true
			r.Builtin = false
			m.customRules[r.ID] = r
		}
	}

	if v, ok := m.cache.Get(errorPolicyDisabledBuiltinKey); ok {
		var ids []string
		decodeAny(v, &ids)
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id != "" {
				m.disabledBuiltin[id] = true
			}
		}
	}

	if v, ok := m.cache.Get(errorPolicyObservedKey); ok {
		var records []errorPolicyObservedRecord
		decodeAny(v, &records)
		for _, r := range records {
			if strings.TrimSpace(r.ID) == "" {
				continue
			}
			m.observed[r.ID] = r
		}
	}
}

func decodeAny(src any, dst any) {
	b, err := json.Marshal(src)
	if err != nil {
		return
	}
	_ = json.Unmarshal(b, dst)
}

func (m *ErrorPolicyManager) Apply(statusCode int, classified ClassifiedError) ClassifiedError {
	if m == nil {
		return classified
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	code := strings.ToLower(strings.TrimSpace(classified.Code))
	msg := strings.ToLower(strings.TrimSpace(classified.Message))

	for _, r := range builtinErrorPolicies() {
		if m.disabledBuiltin[r.ID] {
			continue
		}
		if matchErrorPolicyRule(r, statusCode, code, msg) {
			return applyErrorPolicyAction(classified, r.Action)
		}
	}
	for _, r := range m.customRules {
		if matchErrorPolicyRule(r, statusCode, code, msg) {
			return applyErrorPolicyAction(classified, r.Action)
		}
	}
	return classified
}

func applyErrorPolicyAction(classified ClassifiedError, action ErrorPolicyAction) ClassifiedError {
	switch action {
	case ErrorPolicyActionAutoSwitch:
		classified.Severity = SeverityNodeDown
		classified.Retryable = true
	}
	return classified
}

func matchErrorPolicyRule(rule ErrorPolicyRule, statusCode int, code, msg string) bool {
	if rule.StatusCode > 0 && rule.StatusCode != statusCode {
		return false
	}
	if rule.ErrorCode != "" && !strings.Contains(code, strings.ToLower(rule.ErrorCode)) {
		return false
	}
	if rule.MessageContains != "" && !strings.Contains(msg, strings.ToLower(rule.MessageContains)) {
		return false
	}
	return true
}

func (m *ErrorPolicyManager) Record(statusCode int, code, message string) {
	if m == nil {
		return
	}
	id := makeObservedPolicyID(statusCode, code, message)
	m.mu.Lock()
	rec := m.observed[id]
	if rec.ID == "" {
		rec = errorPolicyObservedRecord{ID: id, StatusCode: statusCode, ErrorCode: strings.TrimSpace(code), Message: trimMessage(message), Count: 0}
	}
	rec.Count++
	rec.LastSeenAt = time.Now()
	m.observed[id] = rec
	trimObservedMap(m.observed)
	snapshot := m.snapshotObservedLocked()
	m.mu.Unlock()
	m.persistObserved(snapshot)
}

func trimMessage(message string) string {
	msg := strings.TrimSpace(message)
	if len(msg) > 240 {
		return msg[:240]
	}
	return msg
}

func makeObservedPolicyID(statusCode int, code, message string) string {
	c := strings.ToLower(strings.TrimSpace(code))
	m := strings.ToLower(trimMessage(message))
	return fmt.Sprintf("%d|%s|%s", statusCode, c, m)
}

func trimObservedMap(observed map[string]errorPolicyObservedRecord) {
	if len(observed) <= maxObservedErrorPolicies {
		return
	}
	type pair struct {
		id string
		t  time.Time
	}
	arr := make([]pair, 0, len(observed))
	for id, r := range observed {
		arr = append(arr, pair{id: id, t: r.LastSeenAt})
	}
	sort.Slice(arr, func(i, j int) bool { return arr[i].t.After(arr[j].t) })
	for i := maxObservedErrorPolicies; i < len(arr); i++ {
		delete(observed, arr[i].id)
	}
}

func (m *ErrorPolicyManager) persistObserved(records []errorPolicyObservedRecord) {
	if m == nil || m.cache == nil {
		return
	}
	_ = m.cache.Set(errorPolicyObservedKey, records)
}

func (m *ErrorPolicyManager) snapshotObservedLocked() []errorPolicyObservedRecord {
	arr := make([]errorPolicyObservedRecord, 0, len(m.observed))
	for _, r := range m.observed {
		arr = append(arr, r)
	}
	sort.Slice(arr, func(i, j int) bool { return arr[i].LastSeenAt.After(arr[j].LastSeenAt) })
	return arr
}

func (m *ErrorPolicyManager) Snapshot() ErrorPolicySnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	builtins := builtinErrorPolicies()
	for i := range builtins {
		if m.disabledBuiltin[builtins[i].ID] {
			builtins[i].Enabled = false
		}
	}

	custom := make([]ErrorPolicyRule, 0, len(m.customRules))
	for _, r := range m.customRules {
		custom = append(custom, r)
	}
	sort.Slice(custom, func(i, j int) bool { return custom[i].ID < custom[j].ID })

	observedRows := make([]ObservedErrorPolicy, 0, len(m.observed))
	for _, o := range m.snapshotObservedLocked() {
		ruleID := customRuleIDFromObserved(o)
		_, customEnabled := m.customRules[ruleID]
		builtinEnabled := m.matchesEnabledBuiltinLocked(o)
		observedRows = append(observedRows, ObservedErrorPolicy{
			ID:         o.ID,
			StatusCode: o.StatusCode,
			ErrorCode:  o.ErrorCode,
			Message:    o.Message,
			Count:      o.Count,
			LastSeenAt: o.LastSeenAt,
			AutoSwitch: customEnabled || builtinEnabled,
		})
	}

	return ErrorPolicySnapshot{BuiltinRules: builtins, CustomRules: custom, Observed: observedRows}
}

func (m *ErrorPolicyManager) matchesEnabledBuiltinLocked(o errorPolicyObservedRecord) bool {
	code := strings.ToLower(strings.TrimSpace(o.ErrorCode))
	msg := strings.ToLower(strings.TrimSpace(o.Message))
	for _, r := range builtinErrorPolicies() {
		if m.disabledBuiltin[r.ID] {
			continue
		}
		if matchErrorPolicyRule(r, o.StatusCode, code, msg) {
			return true
		}
	}
	return false
}

func customRuleIDFromObserved(o errorPolicyObservedRecord) string {
	return "custom-observed-" + o.ID
}

func (m *ErrorPolicyManager) SetBuiltinRuleEnabled(ruleID string, enabled bool) {
	if m == nil {
		return
	}
	m.mu.Lock()
	if enabled {
		delete(m.disabledBuiltin, ruleID)
	} else {
		m.disabledBuiltin[ruleID] = true
	}
	disabled := make([]string, 0, len(m.disabledBuiltin))
	for id := range m.disabledBuiltin {
		disabled = append(disabled, id)
	}
	sort.Strings(disabled)
	m.mu.Unlock()

	if m.cache != nil {
		_ = m.cache.Set(errorPolicyDisabledBuiltinKey, disabled)
	}
}

func (m *ErrorPolicyManager) SetObservedAutoSwitch(observedID string, enabled bool) {
	if m == nil {
		return
	}
	m.mu.Lock()
	o, ok := m.observed[observedID]
	if !ok {
		m.mu.Unlock()
		return
	}
	ruleID := customRuleIDFromObserved(o)
	if enabled {
		m.customRules[ruleID] = ErrorPolicyRule{
			ID:              ruleID,
			Name:            "Observed " + observedID,
			StatusCode:      o.StatusCode,
			ErrorCode:       o.ErrorCode,
			MessageContains: o.Message,
			Action:          ErrorPolicyActionAutoSwitch,
			Enabled:         true,
			Builtin:         false,
		}
	} else {
		delete(m.customRules, ruleID)
	}
	custom := make([]ErrorPolicyRule, 0, len(m.customRules))
	for _, r := range m.customRules {
		custom = append(custom, r)
	}
	m.mu.Unlock()

	if m.cache != nil {
		_ = m.cache.Set(errorPolicyCustomRulesKey, custom)
	}
}
