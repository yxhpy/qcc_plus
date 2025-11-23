package store

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"time"
)

// TunnelConfig 隧道配置持久化模型。
type TunnelConfig struct {
	ID        string    `json:"id"`
	APIToken  string    `json:"api_token"`
	Subdomain string    `json:"subdomain"`
	Zone      string    `json:"zone"`
	Enabled   bool      `json:"enabled"`
	PublicURL string    `json:"public_url"`
	Status    string    `json:"status"`
	LastError string    `json:"last_error"`
	UpdatedAt time.Time `json:"updated_at"`
}

const (
	defaultTunnelID = "default"
	tokenPrefix     = "enc:"
)

// GetTunnelConfig 读取隧道配置；未配置时返回 ErrNotFound。
func (s *Store) GetTunnelConfig(ctx context.Context) (*TunnelConfig, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	row := s.db.QueryRowContext(ctx, `SELECT id, api_token, subdomain, zone, enabled, public_url, status, last_error, updated_at FROM tunnel_config LIMIT 1`)

	var cfg TunnelConfig
	var token string
	var publicURL, status, lastError sql.NullString
	if err := row.Scan(&cfg.ID, &token, &cfg.Subdomain, &cfg.Zone, &cfg.Enabled, &publicURL, &status, &lastError, &cfg.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	cfg.APIToken = decodeToken(token)
	if publicURL.Valid {
		cfg.PublicURL = publicURL.String
	}
	if status.Valid {
		cfg.Status = status.String
	}
	if lastError.Valid {
		cfg.LastError = lastError.String
	}
	return &cfg, nil
}

// SaveTunnelConfig 保存隧道配置；若未提供 APIToken 则沿用已存储的值。
func (s *Store) SaveTunnelConfig(ctx context.Context, cfg TunnelConfig) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	if cfg.ID == "" {
		cfg.ID = defaultTunnelID
	}

	storedToken := cfg.APIToken
	if storedToken == "" {
		if existing, err := s.GetTunnelConfig(ctx); err == nil && existing != nil {
			storedToken = existing.APIToken
		} else if err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}
	}

	encToken := encodeToken(storedToken)

	_, err := s.db.ExecContext(ctx, `INSERT INTO tunnel_config (id, api_token, subdomain, zone, enabled, public_url, status, last_error, updated_at)
VALUES (?,?,?,?,?,?,?, ?, NOW())
ON DUPLICATE KEY UPDATE api_token=VALUES(api_token), subdomain=VALUES(subdomain), zone=VALUES(zone), enabled=VALUES(enabled), public_url=VALUES(public_url), status=VALUES(status), last_error=VALUES(last_error), updated_at=NOW()`,
		cfg.ID, encToken, cfg.Subdomain, cfg.Zone, cfg.Enabled, nullOrString(cfg.PublicURL), cfg.Status, cfg.LastError)
	return err
}

func encodeToken(token string) string {
	if token == "" {
		return ""
	}
	return tokenPrefix + base64.StdEncoding.EncodeToString([]byte(token))
}

func decodeToken(val string) string {
	if val == "" {
		return ""
	}
	if strings.HasPrefix(val, tokenPrefix) {
		b, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(val, tokenPrefix))
		if err == nil {
			return string(b)
		}
	}
	return val
}
