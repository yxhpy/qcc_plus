package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"qcc_plus/internal/store"
)

func TestHandleMonitorDashboard(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		query         string
		setupAccount  bool
		setupNode     bool
		isAdmin       bool
		wantStatus    int
		checkResponse func(*testing.T, *MonitorDashboardResponse)
	}{
		{
			name:       "method not allowed",
			method:     http.MethodPost,
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "unauthorized - no account",
			method:     http.MethodGet,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:         "success - get own dashboard",
			method:       http.MethodGet,
			setupAccount: true,
			setupNode:    true,
			wantStatus:   http.StatusOK,
			checkResponse: func(t *testing.T, resp *MonitorDashboardResponse) {
				if resp.AccountID == "" {
					t.Error("expected account_id in response")
				}
				if len(resp.Nodes) == 0 {
					t.Error("expected at least one node")
				}
			},
		},
		{
			name:         "success - admin get other account dashboard",
			method:       http.MethodGet,
			query:        "?account_id=other-account",
			setupAccount: true,
			isAdmin:      true,
			wantStatus:   http.StatusNotFound, // other-account doesn't exist
		},
		{
			name:         "forbidden - non-admin get other account",
			method:       http.MethodGet,
			query:        "?account_id=other-account",
			setupAccount: true,
			isAdmin:      false,
			wantStatus:   http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := buildServerNoWarmup(t, NewBuilder().WithUpstream("http://example.com"))

			var acc *Account
			if tt.setupAccount {
				var err error
				acc, err = srv.createAccount("test-account", "test-key", "test123", tt.isAdmin)
				if err != nil {
					t.Fatalf("create account: %v", err)
				}
			}

			if tt.setupNode && acc != nil {
				_, err := srv.addNodeToAccount(acc, "node-1", "http://example.com", "key1", 1)
				if err != nil {
					t.Fatalf("add node: %v", err)
				}
			}

			req := httptest.NewRequest(tt.method, "/api/monitor"+tt.query, nil)
			if acc != nil {
				ctx := context.WithValue(req.Context(), accountContextKey{}, acc)
				if tt.isAdmin {
					ctx = withAdmin(ctx, true)
				}
				req = req.WithContext(ctx)
			}

			w := httptest.NewRecorder()
			srv.handleMonitorDashboard(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.checkResponse != nil && w.Code == http.StatusOK {
				var resp MonitorDashboardResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				tt.checkResponse(t, &resp)
			}
		})
	}
}

func TestBuildMonitorDashboardResponse(t *testing.T) {
	srv := buildServerNoWarmup(t, NewBuilder().WithUpstream("http://example.com"))

	acc, err := srv.createAccount("test-account", "test-key", "test123", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	node1, err := srv.addNodeToAccount(acc, "node-1", "http://example.com", "key1", 1)
	if err != nil {
		t.Fatalf("add node 1: %v", err)
	}

	node2, err := srv.addNodeToAccount(acc, "node-2", "http://example.com", "key2", 2)
	if err != nil {
		t.Fatalf("add node 2: %v", err)
	}

	// Disable node2
	srv.mu.Lock()
	acc.Nodes[node2.ID].Disabled = true
	srv.mu.Unlock()

	resp := srv.buildMonitorDashboardResponse(context.Background(), acc)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}

	if resp.AccountID != acc.ID {
		t.Errorf("account_id = %v, want %v", resp.AccountID, acc.ID)
	}

	if resp.AccountName != acc.Name {
		t.Errorf("account_name = %v, want %v", resp.AccountName, acc.Name)
	}

	// Should have 2 nodes (disabled nodes are hidden by default)
	if len(resp.Nodes) != 1 {
		t.Errorf("nodes count = %d, want 1 (disabled node should be hidden)", len(resp.Nodes))
	}

	// Check node details
	if len(resp.Nodes) > 0 {
		n := resp.Nodes[0]
		if n.ID != node1.ID {
			t.Errorf("node id = %v, want %v", n.ID, node1.ID)
		}
		if n.Name != node1.Name {
			t.Errorf("node name = %v, want %v", n.Name, node1.Name)
		}
		if n.Weight != node1.Weight {
			t.Errorf("node weight = %v, want %v", n.Weight, node1.Weight)
		}
	}
}

func TestBuildMonitorDashboardResponse_NilAccount(t *testing.T) {
	srv := buildServerNoWarmup(t, NewBuilder().WithUpstream("http://example.com"))
	resp := srv.buildMonitorDashboardResponse(context.Background(), nil)
	if resp != nil {
		t.Error("expected nil response for nil account")
	}
}

func TestCalculateSuccessRate(t *testing.T) {
	tests := []struct {
		success int64
		fail    int64
		want    float64
	}{
		{100, 0, 100.0},
		{90, 10, 90.0},
		{50, 50, 50.0},
		{0, 100, 0.0},
		{0, 0, 100.0},
		{1, 0, 100.0},
		{0, 1, 0.0},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := calculateSuccessRate(tt.success, tt.fail)
			if got != tt.want {
				t.Errorf("calculateSuccessRate(%d, %d) = %v, want %v", tt.success, tt.fail, got, tt.want)
			}
		})
	}
}

func TestCalculateAvgResponseTime(t *testing.T) {
	tests := []struct {
		sumMS int64
		count int64
		want  int64
	}{
		{1000, 10, 100},
		{0, 10, 0},
		{1000, 0, 0},
		{0, 0, 0},
		{500, 5, 100},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := calculateAvgResponseTime(tt.sumMS, tt.count)
			if got != tt.want {
				t.Errorf("calculateAvgResponseTime(%d, %d) = %v, want %v", tt.sumMS, tt.count, got, tt.want)
			}
		})
	}
}

func TestBuildTrendPoints(t *testing.T) {
	now := time.Now()
	records := []store.MetricsRecord{
		{
			Timestamp:         now.Add(-2 * time.Hour),
			RequestsSuccess:   90,
			RequestsFailed:    10,
			ResponseTimeSumMs: 1000,
			ResponseTimeCount: 10,
		},
		{
			Timestamp:         now.Add(-1 * time.Hour),
			RequestsSuccess:   95,
			RequestsFailed:    5,
			ResponseTimeSumMs: 500,
			ResponseTimeCount: 5,
		},
	}

	points := buildTrendPoints(records)
	if len(points) != 2 {
		t.Fatalf("points count = %d, want 2", len(points))
	}

	// Check first point
	if points[0].SuccessRate != 90.0 {
		t.Errorf("point[0].SuccessRate = %v, want 90.0", points[0].SuccessRate)
	}
	if points[0].AvgTime != 100 {
		t.Errorf("point[0].AvgTime = %v, want 100", points[0].AvgTime)
	}

	// Check second point
	if points[1].SuccessRate != 95.0 {
		t.Errorf("point[1].SuccessRate = %v, want 95.0", points[1].SuccessRate)
	}
	if points[1].AvgTime != 100 {
		t.Errorf("point[1].AvgTime = %v, want 100", points[1].AvgTime)
	}
}

func TestSummarizeTraffic(t *testing.T) {
	tests := []struct {
		name    string
		metrics metrics
		want    ProxySummary
	}{
		{
			name: "normal traffic",
			metrics: metrics{
				Requests:     100,
				FailCount:    10,
				FirstByteDur: 500 * time.Millisecond,
				StreamDur:    500 * time.Millisecond,
			},
			want: ProxySummary{
				SuccessRate:     90.0,
				AvgResponseTime: 10,
				TotalRequests:   100,
				FailedRequests:  10,
			},
		},
		{
			name: "all success",
			metrics: metrics{
				Requests:     50,
				FailCount:    0,
				FirstByteDur: 1000 * time.Millisecond,
				StreamDur:    1000 * time.Millisecond,
			},
			want: ProxySummary{
				SuccessRate:     100.0,
				AvgResponseTime: 40,
				TotalRequests:   50,
				FailedRequests:  0,
			},
		},
		{
			name: "all failed",
			metrics: metrics{
				Requests:     20,
				FailCount:    20,
				FirstByteDur: 0,
				StreamDur:    0,
			},
			want: ProxySummary{
				SuccessRate:     0.0,
				AvgResponseTime: 0,
				TotalRequests:   20,
				FailedRequests:  20,
			},
		},
		{
			name: "no requests",
			metrics: metrics{
				Requests:     0,
				FailCount:    0,
				FirstByteDur: 0,
				StreamDur:    0,
			},
			want: ProxySummary{
				SuccessRate:     100.0,
				AvgResponseTime: 0,
				TotalRequests:   0,
				FailedRequests:  0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := summarizeTraffic(tt.metrics)
			if got.SuccessRate != tt.want.SuccessRate {
				t.Errorf("SuccessRate = %v, want %v", got.SuccessRate, tt.want.SuccessRate)
			}
			if got.AvgResponseTime != tt.want.AvgResponseTime {
				t.Errorf("AvgResponseTime = %v, want %v", got.AvgResponseTime, tt.want.AvgResponseTime)
			}
			if got.TotalRequests != tt.want.TotalRequests {
				t.Errorf("TotalRequests = %v, want %v", got.TotalRequests, tt.want.TotalRequests)
			}
			if got.FailedRequests != tt.want.FailedRequests {
				t.Errorf("FailedRequests = %v, want %v", got.FailedRequests, tt.want.FailedRequests)
			}
		})
	}
}

func TestSummarizeHealth(t *testing.T) {
	now := time.Now()
	interval := 10 * time.Minute

	tests := []struct {
		name     string
		metrics  metrics
		method   string
		interval time.Duration
		now      time.Time
		want     HealthSummary
	}{
		{
			name: "healthy node",
			metrics: metrics{
				LastHealthCheckAt: now.Add(-5 * time.Minute),
				LastPingMS:        100,
				LastPingErr:       "",
			},
			method:   HealthCheckMethodAPI,
			interval: interval,
			now:      now,
			want: HealthSummary{
				Status:      "up",
				LastPingMs:  100,
				LastPingErr: "",
				CheckMethod: "api",
			},
		},
		{
			name: "down node",
			metrics: metrics{
				LastHealthCheckAt: now.Add(-5 * time.Minute),
				LastPingMS:        0,
				LastPingErr:       "connection refused",
			},
			method:   HealthCheckMethodAPI,
			interval: interval,
			now:      now,
			want: HealthSummary{
				Status:      "down",
				LastPingMs:  0,
				LastPingErr: "connection refused",
				CheckMethod: "api",
			},
		},
		{
			name: "stale node",
			metrics: metrics{
				LastHealthCheckAt: now.Add(-25 * time.Minute),
				LastPingMS:        100,
				LastPingErr:       "",
			},
			method:   HealthCheckMethodAPI,
			interval: interval,
			now:      now,
			want: HealthSummary{
				Status:      "stale",
				LastPingMs:  100,
				LastPingErr: "",
				CheckMethod: "api",
			},
		},
		{
			name: "CLI method",
			metrics: metrics{
				LastHealthCheckAt: now.Add(-5 * time.Minute),
				LastPingMS:        5000,
				LastPingErr:       "",
			},
			method:   HealthCheckMethodCLI,
			interval: interval,
			now:      now,
			want: HealthSummary{
				Status:      "up",
				LastPingMs:  5000,
				LastPingErr: "",
				CheckMethod: "cli",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := summarizeHealth(tt.metrics, tt.method, tt.interval, tt.now)
			if got.Status != tt.want.Status {
				t.Errorf("Status = %v, want %v", got.Status, tt.want.Status)
			}
			if got.LastPingMs != tt.want.LastPingMs {
				t.Errorf("LastPingMs = %v, want %v", got.LastPingMs, tt.want.LastPingMs)
			}
			if got.LastPingErr != tt.want.LastPingErr {
				t.Errorf("LastPingErr = %v, want %v", got.LastPingErr, tt.want.LastPingErr)
			}
			if got.CheckMethod != tt.want.CheckMethod {
				t.Errorf("CheckMethod = %v, want %v", got.CheckMethod, tt.want.CheckMethod)
			}
		})
	}
}

func TestComputeHealthStatus(t *testing.T) {
	now := time.Now()
	interval := 10 * time.Minute

	tests := []struct {
		name        string
		lastCheckAt time.Time
		lastPingErr string
		interval    time.Duration
		now         time.Time
		want        string
	}{
		{
			name:        "up - recent check, no error",
			lastCheckAt: now.Add(-5 * time.Minute),
			lastPingErr: "",
			interval:    interval,
			now:         now,
			want:        "up",
		},
		{
			name:        "down - has error",
			lastCheckAt: now.Add(-5 * time.Minute),
			lastPingErr: "error",
			interval:    interval,
			now:         now,
			want:        "down",
		},
		{
			name:        "stale - old check",
			lastCheckAt: now.Add(-25 * time.Minute),
			lastPingErr: "",
			interval:    interval,
			now:         now,
			want:        "stale",
		},
		{
			name:        "up - zero interval",
			lastCheckAt: now.Add(-1 * time.Hour),
			lastPingErr: "",
			interval:    0,
			now:         now,
			want:        "up",
		},
		{
			name:        "up - zero last check",
			lastCheckAt: time.Time{},
			lastPingErr: "",
			interval:    interval,
			now:         now,
			want:        "up",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeHealthStatus(tt.lastCheckAt, tt.lastPingErr, tt.interval, tt.now)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
