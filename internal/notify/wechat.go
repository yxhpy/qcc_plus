package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"qcc_plus/internal/store"
	"qcc_plus/internal/timeutil"
)

type wechatConfig struct {
	WebhookURL string `json:"webhook_url"`
	Token      string `json:"token,omitempty"`
}

type wechatChannel struct {
	cfg    wechatConfig
	client *http.Client
	name   string
}

func newWechatChannel(rec store.NotificationChannelRecord) (NotificationChannel, error) {
	var cfg wechatConfig
	if len(rec.Config) > 0 {
		if err := json.Unmarshal(rec.Config, &cfg); err != nil {
			return nil, fmt.Errorf("parse wechat config: %w", err)
		}
	}
	if cfg.WebhookURL == "" {
		return nil, errors.New("wechat webhook_url required")
	}
	return &wechatChannel{
		cfg:  cfg,
		name: rec.Name,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}, nil
}

func (w *wechatChannel) Send(ctx context.Context, msg NotificationMessage) error {
	if ctx == nil {
		ctx = context.Background()
	}
	body := map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": formatWechatMarkdown(msg),
		},
	}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.cfg.WebhookURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("wechat webhook status %d", resp.StatusCode)
	}
	var res struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err == nil {
		if res.ErrCode != 0 {
			return fmt.Errorf("wechat webhook error: %s", res.ErrMsg)
		}
	}
	return nil
}

func formatWechatMarkdown(msg NotificationMessage) string {
	content := msg.Content
	if content == "" {
		content = "_无详细内容_"
	}
	title := msg.Title
	if title == "" {
		title = msg.EventType
	}
	return fmt.Sprintf("**%s**\n> 事件类型：%s\n> 时间：%s\n\n%s", title, msg.EventType, timeutil.FormatBeijingTime(msg.OccurredAt), content)
}
