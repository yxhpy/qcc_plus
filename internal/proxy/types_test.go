package proxy

import (
	"net/url"
	"testing"
	"time"
)

func TestNode_Structure(t *testing.T) {
	nodeURL, _ := url.Parse("https://api.example.com")
	node := &Node{
		ID:                "node1",
		Name:              "Test Node",
		URL:               nodeURL,
		APIKey:            "test-key",
		HealthCheckMethod: "api",
		HealthCheckModel:  "claude-haiku-4-5-20251001",
		AccountID:         "account1",
		CreatedAt:         time.Now(),
		Weight:            1,
		Failed:            false,
		Disabled:          false,
		LastError:         "",
	}

	if node.ID != "node1" {
		t.Errorf("ID = %s, want node1", node.ID)
	}
	if node.Name != "Test Node" {
		t.Errorf("Name = %s, want Test Node", node.Name)
	}
	if node.URL.String() != "https://api.example.com" {
		t.Errorf("URL = %s, want https://api.example.com", node.URL.String())
	}
	if node.APIKey != "test-key" {
		t.Errorf("APIKey = %s, want test-key", node.APIKey)
	}
	if node.HealthCheckMethod != "api" {
		t.Errorf("HealthCheckMethod = %s, want api", node.HealthCheckMethod)
	}
	if node.Weight != 1 {
		t.Errorf("Weight = %d, want 1", node.Weight)
	}
	if node.Failed {
		t.Error("Failed should be false")
	}
	if node.Disabled {
		t.Error("Disabled should be false")
	}
}

func TestMetrics_Structure(t *testing.T) {
	m := metrics{
		Requests:          100,
		StreamDur:         10 * time.Second,
		FirstByteDur:      1 * time.Second,
		TotalInputTokens:  1000,
		TotalOutputTokens: 2000,
		TotalBytes:        1024 * 1024,
		LastPingMS:        50,
		LastPingErr:       "",
		LastHealthCheckAt: time.Now(),
		FailCount:         5,
		FailStreak:        0,
	}

	if m.Requests != 100 {
		t.Errorf("Requests = %d, want 100", m.Requests)
	}
	if m.StreamDur != 10*time.Second {
		t.Errorf("StreamDur = %v, want 10s", m.StreamDur)
	}
	if m.FirstByteDur != 1*time.Second {
		t.Errorf("FirstByteDur = %v, want 1s", m.FirstByteDur)
	}
	if m.TotalInputTokens != 1000 {
		t.Errorf("TotalInputTokens = %d, want 1000", m.TotalInputTokens)
	}
	if m.TotalOutputTokens != 2000 {
		t.Errorf("TotalOutputTokens = %d, want 2000", m.TotalOutputTokens)
	}
	if m.TotalBytes != 1024*1024 {
		t.Errorf("TotalBytes = %d, want %d", m.TotalBytes, 1024*1024)
	}
	if m.LastPingMS != 50 {
		t.Errorf("LastPingMS = %d, want 50", m.LastPingMS)
	}
	if m.FailCount != 5 {
		t.Errorf("FailCount = %d, want 5", m.FailCount)
	}
	if m.FailStreak != 0 {
		t.Errorf("FailStreak = %d, want 0", m.FailStreak)
	}
}

func TestUsage_Structure(t *testing.T) {
	u := usage{
		input:     100,
		output:    200,
		modelID:   "claude-3-opus-20240229",
		requestID: "req-123",
	}

	if u.input != 100 {
		t.Errorf("input = %d, want 100", u.input)
	}
	if u.output != 200 {
		t.Errorf("output = %d, want 200", u.output)
	}
	if u.modelID != "claude-3-opus-20240229" {
		t.Errorf("modelID = %s, want claude-3-opus-20240229", u.modelID)
	}
	if u.requestID != "req-123" {
		t.Errorf("requestID = %s, want req-123", u.requestID)
	}
}

func TestConfig_Structure(t *testing.T) {
	cfg := Config{
		Retries:     3,
		FailLimit:   5,
		HealthEvery: 30 * time.Second,
	}

	if cfg.Retries != 3 {
		t.Errorf("Retries = %d, want 3", cfg.Retries)
	}
	if cfg.FailLimit != 5 {
		t.Errorf("FailLimit = %d, want 5", cfg.FailLimit)
	}
	if cfg.HealthEvery != 30*time.Second {
		t.Errorf("HealthEvery = %v, want 30s", cfg.HealthEvery)
	}
}

func TestAccount_Structure(t *testing.T) {
	acc := &Account{
		ID:          "account1",
		Name:        "Test Account",
		Password:    "password123",
		ProxyAPIKey: "proxy-key",
		IsAdmin:     true,
		Nodes:       make(map[string]*Node),
		ActiveID:    "node1",
		Config: Config{
			Retries:     3,
			FailLimit:   5,
			HealthEvery: 30 * time.Second,
		},
		FailedSet: make(map[string]struct{}),
	}

	if acc.ID != "account1" {
		t.Errorf("ID = %s, want account1", acc.ID)
	}
	if acc.Name != "Test Account" {
		t.Errorf("Name = %s, want Test Account", acc.Name)
	}
	if acc.Password != "password123" {
		t.Errorf("Password = %s, want password123", acc.Password)
	}
	if acc.ProxyAPIKey != "proxy-key" {
		t.Errorf("ProxyAPIKey = %s, want proxy-key", acc.ProxyAPIKey)
	}
	if !acc.IsAdmin {
		t.Error("IsAdmin should be true")
	}
	if acc.ActiveID != "node1" {
		t.Errorf("ActiveID = %s, want node1", acc.ActiveID)
	}
	if acc.Nodes == nil {
		t.Error("Nodes should not be nil")
	}
	if acc.FailedSet == nil {
		t.Error("FailedSet should not be nil")
	}
}

func TestAccount_FailedSet(t *testing.T) {
	acc := &Account{
		ID:        "account1",
		FailedSet: make(map[string]struct{}),
	}

	// Test adding to failed set
	acc.FailedSet["node1"] = struct{}{}
	if _, exists := acc.FailedSet["node1"]; !exists {
		t.Error("node1 should be in FailedSet")
	}

	// Test removing from failed set
	delete(acc.FailedSet, "node1")
	if _, exists := acc.FailedSet["node1"]; exists {
		t.Error("node1 should not be in FailedSet after deletion")
	}
}

func TestTunnelStatus_Structure(t *testing.T) {
	status := TunnelStatus{
		APITokenSet: true,
		Subdomain:   "my-proxy",
		Zone:        "example.com",
		Enabled:     true,
		PublicURL:   "https://my-proxy.example.com",
		Status:      "running",
		LastError:   "",
	}

	if !status.APITokenSet {
		t.Error("APITokenSet should be true")
	}
	if status.Subdomain != "my-proxy" {
		t.Errorf("Subdomain = %s, want my-proxy", status.Subdomain)
	}
	if status.Zone != "example.com" {
		t.Errorf("Zone = %s, want example.com", status.Zone)
	}
	if !status.Enabled {
		t.Error("Enabled should be true")
	}
	if status.PublicURL != "https://my-proxy.example.com" {
		t.Errorf("PublicURL = %s, want https://my-proxy.example.com", status.PublicURL)
	}
	if status.Status != "running" {
		t.Errorf("Status = %s, want running", status.Status)
	}
	if status.LastError != "" {
		t.Errorf("LastError = %s, want empty", status.LastError)
	}
}

func TestUsageContextKey(t *testing.T) {
	// Test that usageContextKey is a valid type
	var key usageContextKey
	_ = key // Use the variable to avoid unused error
}

func TestNode_DefaultValues(t *testing.T) {
	node := &Node{}

	if node.ID != "" {
		t.Error("default ID should be empty")
	}
	if node.Name != "" {
		t.Error("default Name should be empty")
	}
	if node.URL != nil {
		t.Error("default URL should be nil")
	}
	if node.APIKey != "" {
		t.Error("default APIKey should be empty")
	}
	if node.Weight != 0 {
		t.Error("default Weight should be 0")
	}
	if node.Failed {
		t.Error("default Failed should be false")
	}
	if node.Disabled {
		t.Error("default Disabled should be false")
	}
}

func TestMetrics_DefaultValues(t *testing.T) {
	m := metrics{}

	if m.Requests != 0 {
		t.Error("default Requests should be 0")
	}
	if m.StreamDur != 0 {
		t.Error("default StreamDur should be 0")
	}
	if m.FirstByteDur != 0 {
		t.Error("default FirstByteDur should be 0")
	}
	if m.TotalInputTokens != 0 {
		t.Error("default TotalInputTokens should be 0")
	}
	if m.TotalOutputTokens != 0 {
		t.Error("default TotalOutputTokens should be 0")
	}
	if m.TotalBytes != 0 {
		t.Error("default TotalBytes should be 0")
	}
	if m.LastPingMS != 0 {
		t.Error("default LastPingMS should be 0")
	}
	if m.FailCount != 0 {
		t.Error("default FailCount should be 0")
	}
	if m.FailStreak != 0 {
		t.Error("default FailStreak should be 0")
	}
}

func TestConfig_DefaultValues(t *testing.T) {
	cfg := Config{}

	if cfg.Retries != 0 {
		t.Error("default Retries should be 0")
	}
	if cfg.FailLimit != 0 {
		t.Error("default FailLimit should be 0")
	}
	if cfg.HealthEvery != 0 {
		t.Error("default HealthEvery should be 0")
	}
}

func TestAccount_NodesMap(t *testing.T) {
	acc := &Account{
		Nodes: make(map[string]*Node),
	}

	// Test adding nodes
	nodeURL, _ := url.Parse("https://api.example.com")
	node1 := &Node{ID: "node1", Name: "Node 1", URL: nodeURL}
	node2 := &Node{ID: "node2", Name: "Node 2", URL: nodeURL}

	acc.Nodes["node1"] = node1
	acc.Nodes["node2"] = node2

	if len(acc.Nodes) != 2 {
		t.Errorf("Nodes length = %d, want 2", len(acc.Nodes))
	}

	// Test retrieving nodes
	if acc.Nodes["node1"] != node1 {
		t.Error("node1 not found in Nodes map")
	}
	if acc.Nodes["node2"] != node2 {
		t.Error("node2 not found in Nodes map")
	}

	// Test deleting nodes
	delete(acc.Nodes, "node1")
	if len(acc.Nodes) != 1 {
		t.Errorf("Nodes length = %d, want 1 after deletion", len(acc.Nodes))
	}
	if _, exists := acc.Nodes["node1"]; exists {
		t.Error("node1 should not exist after deletion")
	}
}
