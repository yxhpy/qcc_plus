package proxy

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Session 表示一次登录会话。
type Session struct {
	Token     string
	AccountID string
	IsAdmin   bool
	CreatedAt time.Time
	ExpiresAt time.Time
}

// SessionManager 管理用户会话，使用内存同步 Map 存储。
type SessionManager struct {
	sessions sync.Map
	ttl      time.Duration
}

const defaultSessionTTL = 24 * time.Hour

// NewSessionManager 创建会话管理器，ttl<=0 时使用默认 24h。
func NewSessionManager(ttl time.Duration) *SessionManager {
	if ttl <= 0 {
		ttl = defaultSessionTTL
	}
	return &SessionManager{ttl: ttl}
}

// Create 新建会话并返回会话信息。
func (m *SessionManager) Create(accountID string, isAdmin bool) *Session {
	if m == nil {
		return nil
	}
	token := randomToken(32)
	now := time.Now()
	sess := &Session{
		Token:     token,
		AccountID: accountID,
		IsAdmin:   isAdmin,
		CreatedAt: now,
		ExpiresAt: now.Add(m.ttl),
	}
	m.sessions.Store(token, sess)
	return sess
}

// Get 根据 token 读取会话，过期会自动删除。
func (m *SessionManager) Get(token string) *Session {
	if m == nil || token == "" {
		return nil
	}
	if v, ok := m.sessions.Load(token); ok {
		if sess, ok2 := v.(*Session); ok2 {
			if time.Now().After(sess.ExpiresAt) {
				m.sessions.Delete(token)
				return nil
			}
			return sess
		}
	}
	return nil
}

// Delete 删除指定 token 的会话。
func (m *SessionManager) Delete(token string) {
	if m == nil || token == "" {
		return
	}
	m.sessions.Delete(token)
}

// Validate 判断 token 是否仍然有效。
func (m *SessionManager) Validate(token string) bool {
	return m.Get(token) != nil
}

func randomToken(n int) string {
	if n <= 0 {
		n = 32
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// rand.Read 理论上不会失败；失败时退回时间戳。
		return hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(b)
}
