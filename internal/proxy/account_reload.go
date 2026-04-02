package proxy

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"qcc_plus/internal/store"
)

func (p *Server) nodeFromRecord(rec store.NodeRecord) *Node {
	u, _ := url.Parse(rec.BaseURL)
	healthMethod := normalizeHealthCheckMethod(chooseNonEmpty(rec.HealthCheckMethod, defaultHealthCheckMethod))
	protocol := chooseNonEmpty(rec.SourceProtocol, SourceProtocolClaude)
	healthModel := effectiveHealthCheckModelForProtocol(protocol, rec.HealthCheckModel)
	if fixedMethod, fixed := protocolFixedHealthCheckMethod(protocol); fixed {
		healthMethod = fixedMethod
	}

	joinedAPIKey, keyItems, keyRotator := buildNodeKeyState(rec.APIKey, rec.APIKeyConfig)
	if healthMethodRequiresAPIKey(healthMethod) && joinedAPIKey == "" {
		if p != nil && p.logger != nil {
			p.logger.Printf("health check mode %s requires api key, fallback to head for node %s", healthMethod, rec.Name)
		}
		healthMethod = HealthCheckMethodHEAD
	}

	return &Node{
		ID:                rec.ID,
		Name:              rec.Name,
		URL:               u,
		APIKey:            joinedAPIKey,
		APIKeyConfig:      encodeNamedAPIKeys(keyItems),
		APIKeyItems:       cloneNamedAPIKeys(keyItems),
		APIKeys:           keyRotator,
		HealthCheckMethod: healthMethod,
		HealthCheckModel:  healthModel,
		ModelMapping:      decodeModelMapping(rec.ModelMapping),
		SourceProtocol:    protocol,
		AuthProfile:       rec.AuthProfile,
		Capabilities:      rec.Capabilities,
		AccountID:         rec.AccountID,
		CreatedAt:         rec.CreatedAt,
		Weight:            rec.Weight,
		Failed:            rec.Failed,
		Disabled:          rec.Disabled,
		LastError:         rec.LastError,
		Metrics: metrics{
			Requests:          rec.Requests,
			FailCount:         rec.FailCount,
			FailStreak:        rec.FailStreak,
			TotalBytes:        rec.TotalBytes,
			TotalInputTokens:  rec.TotalInput,
			TotalOutputTokens: rec.TotalOutput,
			StreamDur:         time.Duration(rec.StreamDurMs) * time.Millisecond,
			FirstByteDur:      time.Duration(rec.FirstByteMs) * time.Millisecond,
			LastPingMS:        rec.LastPingMs,
			LastPingErr:       rec.LastPingErr,
			LastHealthCheckAt: rec.LastHealthCheckAt,
		},
	}
}

func (p *Server) reloadAccountNodesFromStore(ctx context.Context, accountID string) error {
	if p == nil || p.store == nil {
		return fmt.Errorf("store not configured")
	}

	acc := p.getAccountByID(accountID)
	if acc == nil {
		return fmt.Errorf("account %s not found", accountID)
	}

	recs, cfgLoaded, activeID, err := p.store.LoadAllByAccount(ctx, accountID)
	if err != nil {
		return err
	}

	newConfig := acc.Config
	if cfgLoaded.Retries > 0 {
		newConfig.Retries = cfgLoaded.Retries
	}
	if cfgLoaded.FailLimit > 0 {
		newConfig.FailLimit = cfgLoaded.FailLimit
	}
	if cfgLoaded.HealthEvery > 0 {
		newConfig.HealthEvery = cfgLoaded.HealthEvery
	}

	newNodes := make(map[string]*Node, len(recs))
	newFailedSet := make(map[string]struct{})
	for _, rec := range recs {
		node := p.nodeFromRecord(rec)
		newNodes[node.ID] = node
		if node.Failed {
			newFailedSet[node.ID] = struct{}{}
		}
	}

	p.mu.Lock()
	for id := range acc.Nodes {
		delete(p.nodeIndex, id)
		delete(p.nodeAccount, id)
	}
	acc.Nodes = newNodes
	acc.FailedSet = newFailedSet
	acc.ActiveID = activeID
	acc.Config = newConfig
	for id, node := range newNodes {
		node.AccountID = acc.ID
		p.nodeIndex[id] = node
		p.nodeAccount[id] = acc
	}
	if acc.ID == store.DefaultAccountID {
		p.defaultAccount = acc
	}
	p.mu.Unlock()

	if len(newNodes) == 0 {
		return nil
	}

	if current := p.getNode(acc.ActiveID); current == nil || current.Disabled || current.Failed {
		if _, err := p.selectBestAndActivate(acc, "cc-switch 导入同步"); err != nil && !errors.Is(err, ErrNoActiveNode) {
			return err
		}
	}
	return nil
}
