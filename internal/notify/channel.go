package notify

import (
	"context"
	"fmt"
	"time"

	"qcc_plus/internal/store"
)

// NotificationMessage 发送给渠道的格式化消息。
type NotificationMessage struct {
	AccountID  string
	EventType  string
	Title      string
	Content    string
	OccurredAt time.Time
}

// NotificationChannel 通知渠道需要实现的接口。
type NotificationChannel interface {
	Send(ctx context.Context, msg NotificationMessage) error
}

// buildChannel 根据渠道记录创建具体实现。
func buildChannel(rec store.NotificationChannelRecord) (NotificationChannel, error) {
	switch rec.ChannelType {
	case ChannelWechatWork, ChannelWechatPersonal:
		return newWechatChannel(rec)
	default:
		return nil, fmt.Errorf("unsupported channel type: %s", rec.ChannelType)
	}
}

// BuildChannel 向外暴露的构造器，便于在不同模块直接创建渠道实例。
func BuildChannel(rec store.NotificationChannelRecord) (NotificationChannel, error) {
	return buildChannel(rec)
}
