package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"qcc_plus/internal/store"
)

func TestHandleGetNodeMetrics(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		path          string
		setupAccount  bool
		setupNode     bool
		isAdmin       bool
		wantStatus    int
		checkResponse func(*testing.T, map[string]interface{})
	}{
		{
			name:       "method not allowed",
			method:     http.MethodPost,
			path:       "/api/nodes/node-1/metrics",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "unauthorized - no account",
			method:     http.MethodGet,
			path:       "/api/nodes/node-1/metrics",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:         "node not found",
			method:       http.MethodGet,
			path:         "/api/nodes/nonexistent/metrics",
			setupAccount: true,
			wantStatus:   http.StatusNotFound,
		},
		{
			name:         "forbidden - non-admin accessing other account node",
			method:       http.MethodGet,
			path:         "/api/nodes/node-1/metrics",
			setupAccount: true,
			setupNode:    true,
			isAdmin:      false,
			wantStatus:   http.StatusForbidden,
		},
		{
			name:         "success - get node metrics",
			method:       http.MethodGet,
			path:         "/api/nodes/node-1/metrics?granularity=raw&limit=10",
			setupAccount: true,
			setupNode:    true,
			isAdmin:      true,
			wantStatus:   http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if _, ok := resp["data"]; !ok {
					t.Error("expected 'data' field in response")
				}
				if _, ok := resp["granularity"]; !ok {
					t.Error("expected 'granularity' field in response")
				}
			},
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

			path := tt.path
			if tt.setupNode {
				// For forbidden test: add node to a different account
				nodeOwner := acc
				if !tt.isAdmin && acc != nil {
					otherAcc, err := srv.createAccount("other-account", "other-key", "other123", true)
					if err != nil {
						t.Fatalf("create other account: %v", err)
					}
					nodeOwner = otherAcc
				}
				if nodeOwner != nil {
					node, err := srv.addNodeToAccount(nodeOwner, "node-1", "http://example.com", "key1", 1)
					if err != nil {
						t.Fatalf("add node: %v", err)
					}
					// Replace "node-1" in path with actual node ID
					path = strings.Replace(path, "node-1", node.ID, 1)
				}
			}

			req := httptest.NewRequest(tt.method, path, nil)
			if acc != nil {
				ctx := context.WithValue(req.Context(), accountContextKey{}, acc)
				if tt.isAdmin {
					ctx = withAdmin(ctx, true)
				}
				req = req.WithContext(ctx)
			}

			w := httptest.NewRecorder()
			srv.handleGetNodeMetrics(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.checkResponse != nil && w.Code == http.StatusOK {
				var resp map[string]interface{}
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				tt.checkResponse(t, resp)
			}
		})
	}
}

func TestHandleGetAccountMetrics(t *testing.T) {
	tests := []struct {
		name          string
		method        string
		path          string
		setupAccount  bool
		isAdmin       bool
		wantStatus    int
		checkResponse func(*testing.T, map[string]interface{})
	}{
		{
			name:       "method not allowed",
			method:     http.MethodPost,
			path:       "/api/accounts/acc-1/metrics",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "unauthorized - no account",
			method:     http.MethodGet,
			path:       "/api/accounts/acc-1/metrics",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:         "forbidden - non-admin accessing other account",
			method:       http.MethodGet,
			path:         "/api/accounts/other-acc/metrics",
			setupAccount: true,
			isAdmin:      false,
			wantStatus:   http.StatusForbidden,
		},
		{
			name:         "success - get account metrics",
			method:       http.MethodGet,
			path:         "/api/accounts/test-account/metrics?granularity=hour",
			setupAccount: true,
			isAdmin:      true,
			wantStatus:   http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				if _, ok := resp["data"]; !ok {
					t.Error("expected 'data' field in response")
				}
			},
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

			req := httptest.NewRequest(tt.method, tt.path, nil)
			if acc != nil {
				ctx := context.WithValue(req.Context(), accountContextKey{}, acc)
				if tt.isAdmin {
					ctx = withAdmin(ctx, true)
				}
				req = req.WithContext(ctx)
			}

			w := httptest.NewRecorder()
			srv.handleGetAccountMetrics(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.checkResponse != nil && w.Code == http.StatusOK {
				var resp map[string]interface{}
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				tt.checkResponse(t, resp)
			}
		})
	}
}

func TestHandleAggregateMetrics(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		body       map[string]interface{}
		isAdmin    bool
		wantStatus int
	}{
		{
			name:       "method not allowed",
			method:     http.MethodGet,
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "forbidden - non-admin",
			method:     http.MethodPost,
			isAdmin:    false,
			wantStatus: http.StatusForbidden,
		},
		{
			name:   "invalid json",
			method: http.MethodPost,
			body: map[string]interface{}{
				"target": "invalid",
			},
			isAdmin:    true,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "invalid target",
			method: http.MethodPost,
			body: map[string]interface{}{
				"target": "invalid",
				"from":   time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
				"to":     time.Now().Format(time.RFC3339),
			},
			isAdmin:    true,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "invalid time range",
			method: http.MethodPost,
			body: map[string]interface{}{
				"target": "hour",
				"from":   time.Now().Format(time.RFC3339),
				"to":     time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
			},
			isAdmin:    true,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := buildServerNoWarmup(t, NewBuilder().WithUpstream("http://example.com"))

			var body []byte
			if tt.body != nil {
				body, _ = json.Marshal(tt.body)
			}

			req := httptest.NewRequest(tt.method, "/api/metrics/aggregate", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			if tt.isAdmin {
				ctx := withAdmin(req.Context(), true)
				req = req.WithContext(ctx)
			}

			w := httptest.NewRecorder()
			srv.handleAggregateMetrics(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestHandleCleanupMetrics(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		body       map[string]interface{}
		isAdmin    bool
		wantStatus int
	}{
		{
			name:       "method not allowed",
			method:     http.MethodGet,
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "forbidden - non-admin",
			method:     http.MethodPost,
			isAdmin:    false,
			wantStatus: http.StatusForbidden,
		},
		{
			name:   "invalid json",
			method: http.MethodPost,
			body: map[string]interface{}{
				"invalid": "data",
			},
			isAdmin:    true,
			wantStatus: http.StatusOK, // Will succeed with empty account_id
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := buildServerNoWarmup(t, NewBuilder().WithUpstream("http://example.com"))

			var body []byte
			if tt.body != nil {
				body, _ = json.Marshal(tt.body)
			}

			req := httptest.NewRequest(tt.method, "/api/metrics/cleanup", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			if tt.isAdmin {
				ctx := withAdmin(req.Context(), true)
				req = req.WithContext(ctx)
			}

			w := httptest.NewRecorder()
			srv.handleCleanupMetrics(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestParseMetricsQueryParams(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		wantGran    store.MetricsGranularity
		wantErr     bool
		checkResult func(*testing.T, time.Time, time.Time, int, int)
	}{
		{
			name:     "default values",
			query:    "",
			wantGran: store.MetricsGranularityRaw,
			wantErr:  false,
			checkResult: func(t *testing.T, from, to time.Time, limit, offset int) {
				if limit != 100 {
					t.Errorf("limit = %d, want 100", limit)
				}
				if offset != 0 {
					t.Errorf("offset = %d, want 0", offset)
				}
			},
		},
		{
			name:     "hourly granularity",
			query:    "granularity=hour",
			wantGran: store.MetricsGranularityHourly,
			wantErr:  false,
		},
		{
			name:     "daily granularity",
			query:    "granularity=day",
			wantGran: store.MetricsGranularityDaily,
			wantErr:  false,
		},
		{
			name:     "monthly granularity",
			query:    "granularity=month",
			wantGran: store.MetricsGranularityMonthly,
			wantErr:  false,
		},
		{
			name:    "invalid granularity",
			query:   "granularity=invalid",
			wantErr: true,
		},
		{
			name:    "invalid limit",
			query:   "limit=invalid",
			wantErr: true,
		},
		{
			name:    "invalid offset",
			query:   "offset=invalid",
			wantErr: true,
		},
		{
			name:     "custom limit and offset",
			query:    "limit=50&offset=10",
			wantGran: store.MetricsGranularityRaw,
			wantErr:  false,
			checkResult: func(t *testing.T, from, to time.Time, limit, offset int) {
				if limit != 50 {
					t.Errorf("limit = %d, want 50", limit)
				}
				if offset != 10 {
					t.Errorf("offset = %d, want 10", offset)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test?"+tt.query, nil)
			gran, from, to, limit, offset, err := parseMetricsQueryParams(req)

			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if gran != tt.wantGran {
					t.Errorf("granularity = %v, want %v", gran, tt.wantGran)
				}
				if tt.checkResult != nil {
					tt.checkResult(t, from, to, limit, offset)
				}
			}
		})
	}
}

func TestParseGranularity(t *testing.T) {
	tests := []struct {
		input   string
		want    store.MetricsGranularity
		wantErr bool
	}{
		{"", store.MetricsGranularityRaw, false},
		{"raw", store.MetricsGranularityRaw, false},
		{"hour", store.MetricsGranularityHourly, false},
		{"day", store.MetricsGranularityDaily, false},
		{"month", store.MetricsGranularityMonthly, false},
		{"HOUR", store.MetricsGranularityHourly, false},
		{"invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseGranularity(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseAggregateTarget(t *testing.T) {
	tests := []struct {
		input   string
		want    store.MetricsGranularity
		wantErr bool
	}{
		{"hour", store.MetricsGranularityHourly, false},
		{"day", store.MetricsGranularityDaily, false},
		{"month", store.MetricsGranularityMonthly, false},
		{"HOUR", store.MetricsGranularityHourly, false},
		{"  day  ", store.MetricsGranularityDaily, false},
		{"raw", "", true},
		{"invalid", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseAggregateTarget(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractNodeIDFromPath(t *testing.T) {
	tests := []struct {
		path   string
		wantID string
		wantOK bool
	}{
		{"/api/nodes/node-123/metrics", "node-123", true},
		{"/api/nodes/abc/metrics", "abc", true},
		{"/api/nodes//metrics", "", false},
		{"/api/accounts/acc-1/metrics", "", false},
		{"/invalid/path", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			gotID, gotOK := extractNodeIDFromPath(tt.path)
			if gotID != tt.wantID || gotOK != tt.wantOK {
				t.Errorf("got (%v, %v), want (%v, %v)", gotID, gotOK, tt.wantID, tt.wantOK)
			}
		})
	}
}

func TestExtractAccountIDFromPath(t *testing.T) {
	tests := []struct {
		path   string
		wantID string
		wantOK bool
	}{
		{"/api/accounts/acc-123/metrics", "acc-123", true},
		{"/api/accounts/test/metrics", "test", true},
		{"/api/accounts//metrics", "", false},
		{"/api/nodes/node-1/metrics", "", false},
		{"/invalid/path", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			gotID, gotOK := extractAccountIDFromPath(tt.path)
			if gotID != tt.wantID || gotOK != tt.wantOK {
				t.Errorf("got (%v, %v), want (%v, %v)", gotID, gotOK, tt.wantID, tt.wantOK)
			}
		})
	}
}

func TestSafeDivMetrics(t *testing.T) {
	tests := []struct {
		sum   int64
		count int64
		want  float64
	}{
		{100, 10, 10.0},
		{0, 10, 0.0},
		{100, 0, 0.0},
		{0, 0, 0.0},
		{-100, 10, -10.0},
		{100, -10, 0.0}, // Negative count returns 0
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := safeDiv(tt.sum, tt.count)
			if got != tt.want {
				t.Errorf("safeDiv(%d, %d) = %v, want %v", tt.sum, tt.count, got, tt.want)
			}
		})
	}
}

func TestDefaultFrom(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		gran store.MetricsGranularity
		want time.Duration
	}{
		{store.MetricsGranularityRaw, 24 * time.Hour},
		{store.MetricsGranularityHourly, 7 * 24 * time.Hour},
		{store.MetricsGranularityDaily, 30 * 24 * time.Hour},
		{store.MetricsGranularityMonthly, 365 * 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(string(tt.gran), func(t *testing.T) {
			got := defaultFrom(tt.gran, now)
			expected := now.Add(-tt.want)

			// Allow 1 second tolerance for test execution time
			diff := got.Sub(expected)
			if diff < -time.Second || diff > time.Second {
				t.Errorf("defaultFrom(%v, now) = %v, want approximately %v", tt.gran, got, expected)
			}
		})
	}
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"", false},
		{"2024-01-01T00:00:00Z", false},
		{"2024-12-31T23:59:59Z", false},
		{"invalid", true},
		{"2024-01-01", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseTime(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.input != "" && got.IsZero() {
				t.Error("expected non-zero time")
			}
		})
	}
}
