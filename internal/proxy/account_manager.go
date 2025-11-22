package proxy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"qcc_plus/internal/store"
)

type accountContextKey struct{}
type isAdminContextKey struct{}

func (p *Server) createAccount(name, proxyKey, password string, isAdmin bool) (*Account, error) {
	name = strings.TrimSpace(name)
	proxyKey = strings.TrimSpace(proxyKey)
	password = strings.TrimSpace(password)
	if name == "" {
		return nil, errors.New("name required")
	}
	if proxyKey == "" {
		return nil, errors.New("proxy_api_key required")
	}
	if existing := p.getAccountByProxyKey(proxyKey); existing != nil {
		return nil, fmt.Errorf("proxy_api_key already exists")
	}
	cfg := p.getConfig()
	id := fmt.Sprintf("acc-%d", time.Now().UnixNano())
	acc := &Account{
		ID:          id,
		Name:        name,
		Password:    password,
		ProxyAPIKey: proxyKey,
		IsAdmin:     isAdmin,
		Config:      cfg,
		Nodes:       make(map[string]*Node),
		FailedSet:   make(map[string]struct{}),
	}

	if p.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		now := time.Now()
		if err := p.store.CreateAccount(ctx, store.AccountRecord{
			ID:          acc.ID,
			Name:        acc.Name,
			Password:    acc.Password,
			ProxyAPIKey: acc.ProxyAPIKey,
			IsAdmin:     acc.IsAdmin,
			CreatedAt:   now,
			UpdatedAt:   now,
		}); err != nil {
			return nil, err
		}
		_ = p.store.UpdateConfig(ctx, acc.ID, store.Config{Retries: cfg.Retries, FailLimit: cfg.FailLimit, HealthEvery: cfg.HealthEvery}, "")
	}
	p.registerAccount(acc)
	return acc, nil
}

func accountFromCtx(r *http.Request) *Account {
	if r == nil {
		return nil
	}
	if v := r.Context().Value(accountContextKey{}); v != nil {
		if acc, ok := v.(*Account); ok {
			return acc
		}
	}
	return nil
}

func isAdmin(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	if v := ctx.Value(isAdminContextKey{}); v != nil {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func isAdminCtx(r *http.Request) bool {
	if r == nil {
		return false
	}
	return isAdmin(r.Context())
}

func canManageAccount(ctx context.Context, targetID string) bool {
	if targetID == "" {
		return false
	}
	if isAdmin(ctx) {
		return true
	}
	if v := ctx.Value(accountContextKey{}); v != nil {
		if acc, ok := v.(*Account); ok && acc.ID == targetID {
			return true
		}
	}
	return false
}
