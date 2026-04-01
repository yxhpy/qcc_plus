package proxy

import (
	"context"
	"errors"
	"net/http"
	"time"

	"qcc_plus/internal/store"
)

const sessionLookupTimeout = 3 * time.Second

func (p *Server) resolveSessionAccount(r interface {
	Cookie(name string) (*http.Cookie, error)
}) (*Session, *Account, string) {
	if p == nil || p.sessionMgr == nil {
		return nil, nil, "session manager missing"
	}

	cookie, err := r.Cookie("session_token")
	if err != nil || cookie == nil || cookie.Value == "" {
		return nil, nil, "unauthorized"
	}

	sess := p.sessionMgr.Refresh(cookie.Value)
	if sess == nil {
		return nil, nil, "session invalid"
	}

	acc := p.resolveAccountByID(sess.AccountID)
	if acc == nil && p.defaultAccount != nil && p.defaultAccount.ID == sess.AccountID {
		acc = p.defaultAccount
	}
	if acc == nil {
		p.sessionMgr.Delete(cookie.Value)
		return nil, nil, "account not found"
	}

	return sess, acc, ""
}

func (p *Server) resolveAccountByID(id string) *Account {
	if id == "" {
		return nil
	}
	if acc := p.getAccountByID(id); acc != nil {
		return acc
	}
	if p == nil || p.store == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), sessionLookupTimeout)
	defer cancel()

	rec, err := p.store.GetAccountByID(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		if p.logger != nil {
			p.logger.Printf("restore account %s from store failed: %v", id, err)
		}
		return nil
	}

	cfg := p.getConfig()
	recs, cfgLoaded, activeID, err := p.store.LoadAllByAccount(ctx, rec.ID)
	if err != nil {
		if p.logger != nil {
			p.logger.Printf("restore account %s nodes from store failed: %v", id, err)
		}
		return nil
	}
	if cfgLoaded.Retries > 0 {
		cfg.Retries = cfgLoaded.Retries
	}
	if cfgLoaded.FailLimit > 0 {
		cfg.FailLimit = cfgLoaded.FailLimit
	}
	if cfgLoaded.HealthEvery > 0 {
		cfg.HealthEvery = cfgLoaded.HealthEvery
	}

	password := rec.Password
	if password == "" {
		if rec.ID == store.DefaultAccountID {
			password = "default123"
		} else if rec.IsAdmin {
			password = "admin123"
		}
	}

	acc := &Account{
		ID:          rec.ID,
		Name:        chooseNonEmpty(rec.Name, rec.ID),
		Password:    password,
		ProxyAPIKey: rec.ProxyAPIKey,
		IsAdmin:     rec.IsAdmin,
		Config:      cfg,
		Nodes:       make(map[string]*Node, len(recs)),
		FailedSet:   make(map[string]struct{}),
		ActiveID:    activeID,
	}

	for _, nodeRec := range recs {
		node := p.nodeFromRecord(nodeRec)
		node.AccountID = acc.ID
		acc.Nodes[node.ID] = node
		if node.Failed {
			acc.FailedSet[node.ID] = struct{}{}
		}
	}

	p.registerAccount(acc)
	return p.getAccountByID(id)
}
