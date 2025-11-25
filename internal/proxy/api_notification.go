package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"qcc_plus/internal/notify"
	"qcc_plus/internal/store"
	"qcc_plus/internal/timeutil"
)

// handleNotificationChannels 处理渠道列表与创建。
func (p *Server) handleNotificationChannels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		p.listNotificationChannels(w, r)
	case http.MethodPost:
		p.createNotificationChannel(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleNotificationChannelByID 处理单个渠道的更新与删除。
func (p *Server) handleNotificationChannelByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/notification/channels/")
	id = strings.TrimSuffix(id, "/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}
	switch r.Method {
	case http.MethodPut:
		p.updateNotificationChannel(w, r, id)
	case http.MethodDelete:
		p.deleteNotificationChannel(w, r, id)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleNotificationSubscriptions 处理订阅列表与创建。
func (p *Server) handleNotificationSubscriptions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		p.listNotificationSubscriptions(w, r)
	case http.MethodPost:
		p.createNotificationSubscription(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleNotificationSubscriptionByID 处理订阅更新与删除。
func (p *Server) handleNotificationSubscriptionByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/notification/subscriptions/")
	id = strings.TrimSuffix(id, "/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}
	switch r.Method {
	case http.MethodPut:
		p.updateNotificationSubscription(w, r, id)
	case http.MethodDelete:
		p.deleteNotificationSubscription(w, r, id)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// listNotificationChannels 列出当前账号（或管理员指定账号）的渠道。
func (p *Server) listNotificationChannels(w http.ResponseWriter, r *http.Request) {
	if p.store == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "notification store not enabled"})
		return
	}
	acc := accountFromCtx(r)
	if acc == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	// 管理员可指定 account_id 查看其它账号
	if aid := r.URL.Query().Get("account_id"); aid != "" {
		if !isAdmin(r.Context()) && aid != acc.ID {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		if target := p.getAccountByID(aid); target != nil {
			acc = target
		} else {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "account not found"})
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	recs, err := p.store.ListNotificationChannels(ctx, acc.ID)
	if err != nil {
		p.logger.Printf("list notification channels failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "list channels failed"})
		return
	}
	resp := make([]map[string]interface{}, 0, len(recs))
	for _, rec := range recs {
		resp = append(resp, channelView(rec))
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"channels": resp})
}

// createNotificationChannel 创建渠道。
func (p *Server) createNotificationChannel(w http.ResponseWriter, r *http.Request) {
	if p.store == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "notification store not enabled"})
		return
	}
	acc := accountFromCtx(r)
	if acc == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var req struct {
		Name        string          `json:"name"`
		ChannelType string          `json:"channel_type"`
		Config      json.RawMessage `json:"config"`
		Enabled     *bool           `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if !isSupportedChannel(req.ChannelType) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported channel_type"})
		return
	}
	cfg, err := validateChannelConfig(req.ChannelType, req.Config)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if req.Name == "" {
		req.Name = req.ChannelType
	}
	rec := store.NotificationChannelRecord{
		ID:          fmt.Sprintf("chn-%d", time.Now().UnixNano()),
		AccountID:   acc.ID,
		ChannelType: req.ChannelType,
		Name:        req.Name,
		Config:      cfg,
		Enabled:     enabled,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := p.store.CreateNotificationChannel(context.Background(), rec); err != nil {
		p.logger.Printf("create channel failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "create channel failed"})
		return
	}
	writeJSON(w, http.StatusCreated, channelView(rec))
}

// updateNotificationChannel 更新渠道。
func (p *Server) updateNotificationChannel(w http.ResponseWriter, r *http.Request, id string) {
	if p.store == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "notification store not enabled"})
		return
	}
	caller := accountFromCtx(r)
	if caller == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	rec, err := p.store.GetNotificationChannel(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "channel not found"})
			return
		}
		p.logger.Printf("get channel failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "get channel failed"})
		return
	}
	if !isAdmin(r.Context()) && rec.AccountID != caller.ID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	var req struct {
		Name        *string         `json:"name"`
		ChannelType *string         `json:"channel_type"`
		Config      json.RawMessage `json:"config"`
		Enabled     *bool           `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.ChannelType != nil {
		if !isSupportedChannel(*req.ChannelType) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported channel_type"})
			return
		}
		rec.ChannelType = *req.ChannelType
	}
	if len(req.Config) > 0 {
		cfg, err := validateChannelConfig(rec.ChannelType, req.Config)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		rec.Config = cfg
	}
	if req.Name != nil {
		rec.Name = *req.Name
	}
	if req.Enabled != nil {
		rec.Enabled = *req.Enabled
	}
	rec.UpdatedAt = time.Now()
	// 保持渠道归属不变
	if !isAdmin(r.Context()) {
		rec.AccountID = caller.ID
	}
	if err := p.store.UpdateNotificationChannel(context.Background(), *rec); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "channel not found"})
			return
		}
		p.logger.Printf("update channel failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "update failed"})
		return
	}
	writeJSON(w, http.StatusOK, channelView(*rec))
}

// deleteNotificationChannel 删除渠道及其订阅。
func (p *Server) deleteNotificationChannel(w http.ResponseWriter, r *http.Request, id string) {
	if p.store == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "notification store not enabled"})
		return
	}
	caller := accountFromCtx(r)
	if caller == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	rec, err := p.store.GetNotificationChannel(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "channel not found"})
			return
		}
		p.logger.Printf("get channel failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "get channel failed"})
		return
	}
	if !isAdmin(r.Context()) && rec.AccountID != caller.ID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if err := p.store.DeleteNotificationChannel(context.Background(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "channel not found"})
			return
		}
		p.logger.Printf("delete channel failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "delete failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

// listNotificationSubscriptions 列出订阅。
func (p *Server) listNotificationSubscriptions(w http.ResponseWriter, r *http.Request) {
	if p.store == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "notification store not enabled"})
		return
	}
	acc := accountFromCtx(r)
	if acc == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if aid := r.URL.Query().Get("account_id"); aid != "" {
		if !isAdmin(r.Context()) && aid != acc.ID {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		if target := p.getAccountByID(aid); target != nil {
			acc = target
		} else {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "account not found"})
			return
		}
	}
	channelID := r.URL.Query().Get("channel_id")
	if channelID != "" {
		ch, err := p.store.GetNotificationChannel(r.Context(), channelID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "channel not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "get channel failed"})
			return
		}
		if !isAdmin(r.Context()) && ch.AccountID != acc.ID {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		if isAdmin(r.Context()) && ch.AccountID != acc.ID {
			// 管理员查看其它账号时，以渠道归属账号为准
			if target := p.getAccountByID(ch.AccountID); target != nil {
				acc = target
			}
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	recs, err := p.store.ListNotificationSubscriptions(ctx, acc.ID, channelID)
	if err != nil {
		p.logger.Printf("list notification subscriptions failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "list subscriptions failed"})
		return
	}
	resp := make([]map[string]interface{}, 0, len(recs))
	for _, rec := range recs {
		resp = append(resp, subscriptionView(rec))
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"subscriptions": resp})
}

// createNotificationSubscription 支持批量创建订阅。
func (p *Server) createNotificationSubscription(w http.ResponseWriter, r *http.Request) {
	if p.store == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "notification store not enabled"})
		return
	}
	caller := accountFromCtx(r)
	if caller == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var req struct {
		ChannelID  string   `json:"channel_id"`
		EventTypes []string `json:"event_types"`
		Enabled    *bool    `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.ChannelID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "channel_id required"})
		return
	}
	if len(req.EventTypes) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "event_types required"})
		return
	}
	for _, evt := range req.EventTypes {
		if !isValidEventType(evt) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid event_type: %s", evt)})
			return
		}
	}
	ch, err := p.store.GetNotificationChannel(r.Context(), req.ChannelID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "channel not found"})
			return
		}
		p.logger.Printf("get channel failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "get channel failed"})
		return
	}
	if !isAdmin(r.Context()) && ch.AccountID != caller.ID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	now := time.Now()
	resp := make([]map[string]interface{}, 0, len(req.EventTypes))
	for i, evt := range req.EventTypes {
		sid := fmt.Sprintf("sub-%d-%d", now.UnixNano(), i)
		rec := store.NotificationSubscriptionRecord{
			ID:        sid,
			AccountID: ch.AccountID,
			ChannelID: ch.ID,
			EventType: evt,
			Enabled:   enabled,
			CreatedAt: now,
		}
		if err := p.store.UpsertNotificationSubscription(context.Background(), rec); err != nil {
			p.logger.Printf("create subscription failed: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "create subscription failed"})
			return
		}
		resp = append(resp, subscriptionView(rec))
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{"subscriptions": resp})
}

// updateNotificationSubscription 更新订阅状态。
func (p *Server) updateNotificationSubscription(w http.ResponseWriter, r *http.Request, id string) {
	if p.store == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "notification store not enabled"})
		return
	}
	caller := accountFromCtx(r)
	if caller == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	rec, err := p.store.GetNotificationSubscription(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "subscription not found"})
			return
		}
		p.logger.Printf("get subscription failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "get subscription failed"})
		return
	}
	if !isAdmin(r.Context()) && rec.AccountID != caller.ID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.Enabled == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "enabled required"})
		return
	}
	rec.Enabled = *req.Enabled
	rec.UpdatedAt = time.Now()
	if err := p.store.UpsertNotificationSubscription(context.Background(), *rec); err != nil {
		p.logger.Printf("update subscription failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "update failed"})
		return
	}
	writeJSON(w, http.StatusOK, subscriptionView(*rec))
}

// deleteNotificationSubscription 删除订阅。
func (p *Server) deleteNotificationSubscription(w http.ResponseWriter, r *http.Request, id string) {
	if p.store == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "notification store not enabled"})
		return
	}
	caller := accountFromCtx(r)
	if caller == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	rec, err := p.store.GetNotificationSubscription(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "subscription not found"})
			return
		}
		p.logger.Printf("get subscription failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "get subscription failed"})
		return
	}
	if !isAdmin(r.Context()) && rec.AccountID != caller.ID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if err := p.store.DeleteNotificationSubscription(context.Background(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "subscription not found"})
			return
		}
		p.logger.Printf("delete subscription failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "delete failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

// listEventTypes 返回可订阅事件列表。
func (p *Server) listEventTypes(w http.ResponseWriter, r *http.Request) {
	events := eventTypeCatalog()
	resp := make([]map[string]string, 0, len(events))
	for _, e := range events {
		resp = append(resp, map[string]string{
			"type":        e.Type,
			"category":    e.Category,
			"description": e.Description,
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"event_types": resp})
}

// testNotification 立即向指定渠道发送测试通知。
func (p *Server) testNotification(w http.ResponseWriter, r *http.Request) {
	if p.store == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "notification store not enabled"})
		return
	}
	caller := accountFromCtx(r)
	if caller == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var req struct {
		ChannelID string `json:"channel_id"`
		Title     string `json:"title"`
		Content   string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.ChannelID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "channel_id required"})
		return
	}
	chRec, err := p.store.GetNotificationChannel(r.Context(), req.ChannelID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "channel not found"})
			return
		}
		p.logger.Printf("get channel failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "get channel failed"})
		return
	}
	if !isAdmin(r.Context()) && chRec.AccountID != caller.ID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	ch, err := notify.BuildChannel(*chRec)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	msg := notify.NotificationMessage{
		AccountID:  chRec.AccountID,
		EventType:  "test",
		Title:      chooseNonEmpty(req.Title, "测试通知"),
		Content:    chooseNonEmpty(req.Content, "这是一条测试通知"),
		OccurredAt: timeutil.NowBeijing(),
	}
	if err := ch.Send(ctx, msg); err != nil {
		p.logger.Printf("send test notification failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

// channelView 返回对外展示的渠道信息（隐藏敏感配置）。
func channelView(rec store.NotificationChannelRecord) map[string]interface{} {
	return map[string]interface{}{
		"id":           rec.ID,
		"name":         rec.Name,
		"channel_type": rec.ChannelType,
		"enabled":      rec.Enabled,
		"created_at":   timeutil.FormatBeijingTime(rec.CreatedAt),
		"updated_at":   timeutil.FormatBeijingTime(rec.UpdatedAt),
	}
}

// subscriptionView 返回订阅展示结构。
func subscriptionView(rec store.NotificationSubscriptionRecord) map[string]interface{} {
	return map[string]interface{}{
		"id":         rec.ID,
		"channel_id": rec.ChannelID,
		"event_type": rec.EventType,
		"enabled":    rec.Enabled,
		"created_at": timeutil.FormatBeijingTime(rec.CreatedAt),
	}
}

// channel与订阅校验相关辅助函数。
func isSupportedChannel(tp string) bool {
	switch tp {
	case notify.ChannelWechatWork, notify.ChannelWechatPersonal:
		return true
	default:
		return false
	}
}

func validateChannelConfig(channelType string, raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, errors.New("config required")
	}
	switch channelType {
	case notify.ChannelWechatWork, notify.ChannelWechatPersonal:
		var cfg struct {
			WebhookURL string `json:"webhook_url"`
		}
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return nil, fmt.Errorf("invalid config: %w", err)
		}
		if cfg.WebhookURL == "" {
			return nil, errors.New("webhook_url required")
		}
		if err := validateURL(cfg.WebhookURL); err != nil {
			return nil, err
		}
		return raw, nil
	default:
		return nil, fmt.Errorf("unsupported channel_type: %s", channelType)
	}
}

func validateURL(raw string) error {
	if raw == "" {
		return errors.New("webhook_url required")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return errors.New("webhook_url invalid")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("webhook_url must use http or https")
	}
	return nil
}

func isValidEventType(t string) bool {
	for _, e := range eventTypeCatalog() {
		if e.Type == t {
			return true
		}
	}
	return false
}

type eventTypeInfo struct {
	Type        string
	Category    string
	Description string
}

func eventTypeCatalog() []eventTypeInfo {
	return []eventTypeInfo{
		{notify.EventNodeStatusChanged, "node", "节点状态变化"},
		{notify.EventNodeSwitched, "node", "节点切换"},
		{notify.EventNodeFailed, "node", "节点标记失败"},
		{notify.EventNodeRecovered, "node", "节点恢复"},
		{notify.EventNodeAdded, "node", "节点新增"},
		{notify.EventNodeDeleted, "node", "节点删除"},
		{notify.EventNodeUpdated, "node", "节点更新"},
		{notify.EventNodeEnabled, "node", "节点启用"},
		{notify.EventNodeDisabled, "node", "节点禁用"},
		{notify.EventNodeHealthCheckError, "node", "节点健康检查失败"},
		{notify.EventRequestFailed, "request", "请求失败"},
		{notify.EventRequestUpstreamErr, "request", "上游错误"},
		{notify.EventRequestProxyError, "request", "代理错误"},
		{notify.EventAccountQuotaWarning, "account", "账号配额预警"},
		{notify.EventAccountAuthFailed, "account", "账号认证失败"},
		{notify.EventSystemTunnelStarted, "system", "隧道启动"},
		{notify.EventSystemTunnelStopped, "system", "隧道停止"},
		{notify.EventSystemTunnelError, "system", "隧道错误"},
		{notify.EventSystemError, "system", "系统错误"},
	}
}
