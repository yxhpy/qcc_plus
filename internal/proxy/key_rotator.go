package proxy

import (
	"strings"
	"sync"
	"time"
)

// KeyStatus API Key 状态
type KeyStatus int

const (
	KeyStatusActive   KeyStatus = iota // 正常可用
	KeyStatusDisabled                  // 已禁用（认证失败等）
	KeyStatusCooling                   // 冷却中（限流后等待恢复）
)

func (s KeyStatus) String() string {
	switch s {
	case KeyStatusActive:
		return "active"
	case KeyStatusDisabled:
		return "disabled"
	case KeyStatusCooling:
		return "cooling"
	default:
		return "unknown"
	}
}

// KeyEntry 单个 API Key 的状态
type KeyEntry struct {
	Key           string    `json:"key"`
	Status        KeyStatus `json:"status"`
	DisabledAt    time.Time `json:"disabled_at,omitempty"`
	DisableReason string    `json:"disable_reason,omitempty"`
	CoolUntil     time.Time `json:"cool_until,omitempty"`
	UsageCount    int64     `json:"usage_count"`
	FailCount     int64     `json:"fail_count"`
	LastUsedAt    time.Time `json:"last_used_at,omitempty"`
	LastError     string    `json:"last_error,omitempty"`
}

// KeyRotator 管理单个节点的多 API Key 轮换
type KeyRotator struct {
	mu   sync.RWMutex
	keys []*KeyEntry
	idx  int // 当前使用的 key 索引

	// 配置
	cooldownDuration time.Duration // 限流冷却时间，默认 60s
	autoRecoverSec   int64         // 自动恢复检测间隔（秒），默认 300
}

// KeyRotatorConfig 密钥轮换配置
type KeyRotatorConfig struct {
	CooldownDuration time.Duration // 限流冷却时间
	AutoRecoverSec   int64         // 自动恢复间隔（秒）
}

func loadKeyRotatorConfig() KeyRotatorConfig {
	return KeyRotatorConfig{
		CooldownDuration: time.Duration(GetEnvInt("KEY_COOLDOWN_SEC", 60)) * time.Second,
		AutoRecoverSec:   int64(GetEnvInt("KEY_AUTO_RECOVER_SEC", 300)),
	}
}

// NewKeyRotator 从逗号分隔的 key 字符串创建轮换器
func NewKeyRotator(keysStr string, cfg KeyRotatorConfig) *KeyRotator {
	if cfg.CooldownDuration <= 0 {
		cfg.CooldownDuration = 60 * time.Second
	}
	if cfg.AutoRecoverSec <= 0 {
		cfg.AutoRecoverSec = 300
	}

	kr := &KeyRotator{
		cooldownDuration: cfg.CooldownDuration,
		autoRecoverSec:   cfg.AutoRecoverSec,
	}

	// 解析逗号分隔的多个 key
	parts := strings.Split(keysStr, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		kr.keys = append(kr.keys, &KeyEntry{
			Key:    p,
			Status: KeyStatusActive,
		})
	}

	return kr
}

// HasMultipleKeys 是否有多个 key
func (kr *KeyRotator) HasMultipleKeys() bool {
	if kr == nil {
		return false
	}
	kr.mu.RLock()
	defer kr.mu.RUnlock()
	return len(kr.keys) > 1
}

// KeyCount 返回 key 总数
func (kr *KeyRotator) KeyCount() int {
	if kr == nil {
		return 0
	}
	kr.mu.RLock()
	defer kr.mu.RUnlock()
	return len(kr.keys)
}

// ActiveKeyCount 返回可用 key 数量
func (kr *KeyRotator) ActiveKeyCount() int {
	if kr == nil {
		return 0
	}
	kr.mu.RLock()
	defer kr.mu.RUnlock()
	count := 0
	now := time.Now()
	for _, k := range kr.keys {
		if k.Status == KeyStatusActive {
			count++
		} else if k.Status == KeyStatusCooling && now.After(k.CoolUntil) {
			count++ // 冷却已过期，视为可用
		}
	}
	return count
}

// GetCurrentKey 获取当前应使用的 API Key
// 自动跳过已禁用和冷却中的 key
func (kr *KeyRotator) GetCurrentKey() string {
	if kr == nil {
		return ""
	}
	kr.mu.Lock()
	defer kr.mu.Unlock()

	if len(kr.keys) == 0 {
		return ""
	}

	now := time.Now()

	// 先尝试恢复冷却中的 key
	for _, k := range kr.keys {
		if k.Status == KeyStatusCooling && now.After(k.CoolUntil) {
			k.Status = KeyStatusActive
		}
	}

	// 尝试自动恢复长时间禁用的 key（给予重试机会）
	for _, k := range kr.keys {
		if k.Status == KeyStatusDisabled && kr.autoRecoverSec > 0 {
			if now.Sub(k.DisabledAt).Seconds() > float64(kr.autoRecoverSec) {
				k.Status = KeyStatusActive
				k.FailCount = 0
			}
		}
	}

	// 从当前索引开始，找到第一个可用的 key
	n := len(kr.keys)
	for i := 0; i < n; i++ {
		idx := (kr.idx + i) % n
		k := kr.keys[idx]
		if k.Status == KeyStatusActive {
			kr.idx = idx
			k.UsageCount++
			k.LastUsedAt = now
			return k.Key
		}
	}

	// 所有 key 都不可用，返回第一个（让上层处理错误）
	return kr.keys[0].Key
}

// RotateToNext 切换到下一个可用的 key
// 返回新的 key，如果没有可用的返回空字符串
func (kr *KeyRotator) RotateToNext() string {
	if kr == nil {
		return ""
	}
	kr.mu.Lock()
	defer kr.mu.Unlock()

	if len(kr.keys) <= 1 {
		return ""
	}

	now := time.Now()
	n := len(kr.keys)

	// 从下一个索引开始查找
	for i := 1; i < n; i++ {
		idx := (kr.idx + i) % n
		k := kr.keys[idx]

		// 恢复冷却过期的 key
		if k.Status == KeyStatusCooling && now.After(k.CoolUntil) {
			k.Status = KeyStatusActive
		}

		if k.Status == KeyStatusActive {
			kr.idx = idx
			k.UsageCount++
			k.LastUsedAt = now
			return k.Key
		}
	}

	return ""
}

// RecordSuccess 记录 key 使用成功
func (kr *KeyRotator) RecordSuccess(key string) {
	if kr == nil {
		return
	}
	kr.mu.Lock()
	defer kr.mu.Unlock()

	for _, k := range kr.keys {
		if k.Key == key {
			k.FailCount = 0
			k.LastError = ""
			return
		}
	}
}

// RecordFailure 根据错误分类记录 key 失败
// 返回是否应该切换到下一个 key
func (kr *KeyRotator) RecordFailure(key string, classified ClassifiedError) bool {
	if kr == nil {
		return false
	}
	kr.mu.Lock()
	defer kr.mu.Unlock()

	for _, k := range kr.keys {
		if k.Key != key {
			continue
		}

		k.FailCount++
		k.LastError = classified.Message

		switch {
		case classified.Severity.ShouldDisableKey():
			// Key 失效或账号问题：禁用此 key
			k.Status = KeyStatusDisabled
			k.DisabledAt = time.Now()
			k.DisableReason = classified.Message
			return len(kr.keys) > 1 // 有其他 key 时切换

		case classified.Severity == SeverityTransient && classified.HTTPStatus == 429:
			// 限流：进入冷却
			k.Status = KeyStatusCooling
			k.CoolUntil = time.Now().Add(kr.cooldownDuration)
			return len(kr.keys) > 1

		default:
			return false
		}
	}

	return false
}

// DisableKey 手动禁用指定 key
func (kr *KeyRotator) DisableKey(key, reason string) {
	if kr == nil {
		return
	}
	kr.mu.Lock()
	defer kr.mu.Unlock()

	for _, k := range kr.keys {
		if k.Key == key {
			k.Status = KeyStatusDisabled
			k.DisabledAt = time.Now()
			k.DisableReason = reason
			return
		}
	}
}

// EnableKey 手动启用指定 key
func (kr *KeyRotator) EnableKey(key string) {
	if kr == nil {
		return
	}
	kr.mu.Lock()
	defer kr.mu.Unlock()

	for _, k := range kr.keys {
		if k.Key == key {
			k.Status = KeyStatusActive
			k.FailCount = 0
			k.LastError = ""
			return
		}
	}
}

// GetAllKeys 获取所有 key 的状态（脱敏）
func (kr *KeyRotator) GetAllKeys() []KeyEntry {
	if kr == nil {
		return nil
	}
	kr.mu.RLock()
	defer kr.mu.RUnlock()

	result := make([]KeyEntry, len(kr.keys))
	for i, k := range kr.keys {
		result[i] = *k
		// 脱敏
		if len(result[i].Key) > 8 {
			result[i].Key = result[i].Key[:4] + "****" + result[i].Key[len(result[i].Key)-4:]
		} else {
			result[i].Key = "********"
		}
	}
	return result
}

// GetPrimaryKey 获取第一个 key（用于兼容单 key 场景）
func (kr *KeyRotator) GetPrimaryKey() string {
	if kr == nil {
		return ""
	}
	kr.mu.RLock()
	defer kr.mu.RUnlock()
	if len(kr.keys) == 0 {
		return ""
	}
	return kr.keys[0].Key
}

// AllKeysDisabled 是否所有 key 都已禁用
func (kr *KeyRotator) AllKeysDisabled() bool {
	if kr == nil {
		return true
	}
	kr.mu.RLock()
	defer kr.mu.RUnlock()

	now := time.Now()
	for _, k := range kr.keys {
		if k.Status == KeyStatusActive {
			return false
		}
		if k.Status == KeyStatusCooling && now.After(k.CoolUntil) {
			return false
		}
	}
	return true
}
