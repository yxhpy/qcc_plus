package tunnel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const cfBaseURL = "https://api.cloudflare.com/client/v4"

// Client 封装 Cloudflare API 调用。
type Client struct {
	httpClient *http.Client
	apiToken   string
	baseURL    string
}

// NewClient 创建 Cloudflare API 客户端。
func NewClient(apiToken string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		apiToken:   apiToken,
		baseURL:    cfBaseURL,
	}
}

type cfError struct {
	Message string `json:"message"`
}

type cfResponse[T any] struct {
	Success  bool      `json:"success"`
	Errors   []cfError `json:"errors"`
	Messages []string  `json:"messages"`
	Result   T         `json:"result"`
}

func (c *Client) doRequest(ctx context.Context, method, path string, body any, v any) error {
	if c == nil {
		return errors.New("nil cloudflare client")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var bodyReader io.Reader
	if body != nil {
		buf := &bytes.Buffer{}
		if err := json.NewEncoder(buf).Encode(body); err != nil {
			return err
		}
		bodyReader = buf
	}

	fullURL := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("cloudflare api error: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	if v == nil {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return err
	}
	return nil
}

func gatherErrors[T any](resp cfResponse[T]) error {
	if resp.Success {
		return nil
	}
	if len(resp.Errors) == 0 {
		return errors.New("cloudflare api: unknown error")
	}
	msgs := make([]string, 0, len(resp.Errors))
	for _, e := range resp.Errors {
		msgs = append(msgs, e.Message)
	}
	return fmt.Errorf("cloudflare api: %s", strings.Join(msgs, "; "))
}

// ListZones 获取当前账号下的域名列表。
func (c *Client) ListZones(ctx context.Context) ([]Zone, error) {
	var resp cfResponse[[]struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}]
	if err := c.doRequest(ctx, http.MethodGet, "/zones?per_page=100", nil, &resp); err != nil {
		return nil, err
	}
	if err := gatherErrors(resp); err != nil {
		return nil, err
	}
	zones := make([]Zone, 0, len(resp.Result))
	for _, z := range resp.Result {
		zones = append(zones, Zone{ID: z.ID, Name: z.Name})
	}
	return zones, nil
}

// GetAccountID 获取账号 ID（取第一个账号）。
func (c *Client) GetAccountID(ctx context.Context) (string, error) {
	var resp cfResponse[[]struct {
		ID string `json:"id"`
	}]
	if err := c.doRequest(ctx, http.MethodGet, "/accounts?per_page=1", nil, &resp); err != nil {
		return "", err
	}
	if err := gatherErrors(resp); err != nil {
		return "", err
	}
	if len(resp.Result) == 0 {
		return "", errors.New("no cloudflare account found for token")
	}
	return resp.Result[0].ID, nil
}

// CreateTunnel 创建隧道。
func (c *Client) CreateTunnel(ctx context.Context, accountID, name, secret string) (*Tunnel, error) {
	body := map[string]any{
		"name":          name,
		"tunnel_secret": secret,
		"config_src":    "cloudflare",
	}
	var resp cfResponse[struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Token string `json:"tunnel_token"`
	}]
	path := fmt.Sprintf("/accounts/%s/cfd_tunnel", url.PathEscape(accountID))
	if err := c.doRequest(ctx, http.MethodPost, path, body, &resp); err != nil {
		return nil, err
	}
	if err := gatherErrors(resp); err != nil {
		return nil, err
	}
	return &Tunnel{ID: resp.Result.ID, Name: resp.Result.Name, Secret: secret}, nil
}

// DeleteTunnel 删除隧道。
func (c *Client) DeleteTunnel(ctx context.Context, accountID, tunnelID string) error {
	path := fmt.Sprintf("/accounts/%s/cfd_tunnel/%s", url.PathEscape(accountID), url.PathEscape(tunnelID))
	var resp cfResponse[struct{}]
	if err := c.doRequest(ctx, http.MethodDelete, path, nil, &resp); err != nil {
		return err
	}
	return gatherErrors(resp)
}

// GetTunnelToken 获取隧道令牌。
func (c *Client) GetTunnelToken(ctx context.Context, accountID, tunnelID string) (string, error) {
	path := fmt.Sprintf("/accounts/%s/cfd_tunnel/%s/token", url.PathEscape(accountID), url.PathEscape(tunnelID))
	// Cloudflare API 直接返回 token 字符串作为 result
	var resp cfResponse[string]
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return "", err
	}
	if err := gatherErrors(resp); err != nil {
		return "", err
	}
	if resp.Result == "" {
		return "", errors.New("empty tunnel token returned")
	}
	return resp.Result, nil
}

// CreateDNSRecord 创建 CNAME 记录指向 tunnel。
func (c *Client) CreateDNSRecord(ctx context.Context, zoneID, name, tunnelID string) error {
	body := map[string]any{
		"type":    "CNAME",
		"name":    name,
		"content": fmt.Sprintf("%s.cfargotunnel.com", tunnelID),
		"proxied": true,
		"ttl":     120,
	}
	path := fmt.Sprintf("/zones/%s/dns_records", url.PathEscape(zoneID))
	var resp cfResponse[struct {
		ID string `json:"id"`
	}]
	if err := c.doRequest(ctx, http.MethodPost, path, body, &resp); err != nil {
		return err
	}
	return gatherErrors(resp)
}

// DeleteDNSRecord 删除指定的 DNS 记录。
func (c *Client) DeleteDNSRecord(ctx context.Context, zoneID, recordID string) error {
	path := fmt.Sprintf("/zones/%s/dns_records/%s", url.PathEscape(zoneID), url.PathEscape(recordID))
	var resp cfResponse[struct{}]
	if err := c.doRequest(ctx, http.MethodDelete, path, nil, &resp); err != nil {
		return err
	}
	return gatherErrors(resp)
}

// FindDNSRecord 查找指定名称的 CNAME 记录。
func (c *Client) FindDNSRecord(ctx context.Context, zoneID, name string) (*DNSRecord, error) {
	query := url.Values{}
	query.Set("type", "CNAME")
	query.Set("name", name)
	path := fmt.Sprintf("/zones/%s/dns_records?%s", url.PathEscape(zoneID), query.Encode())

	var resp cfResponse[[]struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Name    string `json:"name"`
		Content string `json:"content"`
	}]
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	if err := gatherErrors(resp); err != nil {
		return nil, err
	}
	if len(resp.Result) == 0 {
		return nil, nil
	}
	r := resp.Result[0]
	return &DNSRecord{ID: r.ID, Name: r.Name, Content: r.Content, Type: r.Type}, nil
}

// updateTunnelConfig 设置隧道的反向代理目标。
func (c *Client) updateTunnelConfig(ctx context.Context, accountID, tunnelID, hostname, service string) error {
	body := map[string]any{
		"config": map[string]any{
			"ingress": []map[string]any{
				{
					"hostname": hostname,
					"service":  service,
				},
				{
					"service": "http_status:404",
				},
			},
		},
	}
	path := fmt.Sprintf("/accounts/%s/cfd_tunnel/%s/configurations", url.PathEscape(accountID), url.PathEscape(tunnelID))
	var resp cfResponse[struct{}]
	if err := c.doRequest(ctx, http.MethodPut, path, body, &resp); err != nil {
		return err
	}
	return gatherErrors(resp)
}
