package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMetricsWriter_Header(t *testing.T) {
	recorder := httptest.NewRecorder()
	mw := &metricsWriter{ResponseWriter: recorder}

	header := mw.Header()
	if header == nil {
		t.Error("Header() should return non-nil header")
	}
}

func TestMetricsWriter_WriteHeader(t *testing.T) {
	recorder := httptest.NewRecorder()
	mw := &metricsWriter{ResponseWriter: recorder}

	mw.WriteHeader(http.StatusCreated)
	if mw.status != http.StatusCreated {
		t.Errorf("status = %d, want %d", mw.status, http.StatusCreated)
	}
	if !mw.wroteHeader {
		t.Error("wroteHeader should be true")
	}

	// Test double write (should be ignored)
	mw.WriteHeader(http.StatusOK)
	if mw.status != http.StatusCreated {
		t.Error("status should not change on second WriteHeader call")
	}
}

func TestMetricsWriter_Write(t *testing.T) {
	recorder := httptest.NewRecorder()
	mw := &metricsWriter{ResponseWriter: recorder}

	data := []byte("test data")
	n, err := mw.Write(data)

	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if n != len(data) {
		t.Errorf("Write() n = %d, want %d", n, len(data))
	}
	if !mw.firstWrite {
		t.Error("firstWrite should be true")
	}
	if mw.bytes != int64(len(data)) {
		t.Errorf("bytes = %d, want %d", mw.bytes, len(data))
	}
	if mw.status != http.StatusOK {
		t.Errorf("status = %d, want %d (default)", mw.status, http.StatusOK)
	}
	if mw.firstAt.IsZero() {
		t.Error("firstAt should be set")
	}
	if mw.lastAt.IsZero() {
		t.Error("lastAt should be set")
	}

	// Test second write
	time.Sleep(time.Millisecond)
	_, err = mw.Write(data)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if mw.bytes != int64(len(data)*2) {
		t.Errorf("bytes = %d, want %d", mw.bytes, len(data)*2)
	}
	if !mw.lastAt.After(mw.firstAt) {
		t.Error("lastAt should be after firstAt")
	}
}

func TestBuildMetricsRecord(t *testing.T) {
	start := time.Now()
	end := start.Add(time.Second)

	tests := []struct {
		name string
		mw   *metricsWriter
		u    *usage
	}{
		{
			name: "successful request",
			mw: &metricsWriter{
				firstWrite: true,
				status:     http.StatusOK,
				bytes:      1024,
				firstAt:    start.Add(100 * time.Millisecond),
				lastAt:     end,
			},
			u: &usage{
				input:  100,
				output: 200,
			},
		},
		{
			name: "failed request",
			mw: &metricsWriter{
				firstWrite: true,
				status:     http.StatusInternalServerError,
				bytes:      512,
				firstAt:    start.Add(50 * time.Millisecond),
				lastAt:     end,
			},
			u: nil,
		},
		{
			name: "nil metrics writer",
			mw:   nil,
			u: &usage{
				input:  50,
				output: 75,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := buildMetricsRecord("account1", "node1", start, end, tt.mw, tt.u, 0, 0, "claude")

			if rec == nil {
				t.Fatal("buildMetricsRecord should return non-nil record")
			}
			if rec.AccountID != "account1" {
				t.Errorf("AccountID = %s, want account1", rec.AccountID)
			}
			if rec.NodeID != "node1" {
				t.Errorf("NodeID = %s, want node1", rec.NodeID)
			}
			if rec.RequestsTotal != 1 {
				t.Errorf("RequestsTotal = %d, want 1", rec.RequestsTotal)
			}

			if tt.mw != nil {
				if tt.mw.status == http.StatusOK {
					if rec.RequestsSuccess != 1 {
						t.Errorf("RequestsSuccess = %d, want 1", rec.RequestsSuccess)
					}
					if rec.RequestsFailed != 0 {
						t.Errorf("RequestsFailed = %d, want 0", rec.RequestsFailed)
					}
				} else {
					if rec.RequestsSuccess != 0 {
						t.Errorf("RequestsSuccess = %d, want 0", rec.RequestsSuccess)
					}
					if rec.RequestsFailed != 1 {
						t.Errorf("RequestsFailed = %d, want 1", rec.RequestsFailed)
					}
				}
				if rec.BytesTotal != tt.mw.bytes {
					t.Errorf("BytesTotal = %d, want %d", rec.BytesTotal, tt.mw.bytes)
				}
			}

			if tt.u != nil {
				if rec.InputTokensTotal != tt.u.input {
					t.Errorf("InputTokensTotal = %d, want %d", rec.InputTokensTotal, tt.u.input)
				}
				if rec.OutputTokensTotal != tt.u.output {
					t.Errorf("OutputTokensTotal = %d, want %d", rec.OutputTokensTotal, tt.u.output)
				}
			}
		})
	}
}

func TestParseUsage_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		data       []byte
		wantInput  int64
		wantOutput int64
	}{
		{
			name:       "nested usage",
			data:       []byte(`{"data":{"usage":{"input_tokens":10,"output_tokens":20}}}`),
			wantInput:  10,
			wantOutput: 20,
		},
		{
			name:       "multiple usage fields",
			data:       []byte(`{"usage":{"input_tokens":1,"output_tokens":2}}{"usage":{"input_tokens":3,"output_tokens":4}}`),
			wantInput:  3, // Should use last occurrence
			wantOutput: 4,
		},
		{
			name:       "incomplete usage object",
			data:       []byte(`{"usage":{"input_tokens":100}}`),
			wantInput:  100,
			wantOutput: 0,
		},
		{
			name:       "usage with extra fields",
			data:       []byte(`{"usage":{"input_tokens":50,"output_tokens":60,"cache_tokens":10}}`),
			wantInput:  50,
			wantOutput: 60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, output := parseUsage(tt.data)
			if input != tt.wantInput {
				t.Errorf("parseUsage() input = %d, want %d", input, tt.wantInput)
			}
			if output != tt.wantOutput {
				t.Errorf("parseUsage() output = %d, want %d", output, tt.wantOutput)
			}
		})
	}
}

func TestServer_recordMetrics_NoStore(t *testing.T) {
	srv := &Server{
		nodeIndex:   make(map[string]*Node),
		nodeAccount: make(map[string]*Account),
		store:       nil, // No store
	}

	node := &Node{
		ID:        "node1",
		Name:      "Test Node",
		AccountID: "account1",
		Metrics:   metrics{},
	}
	srv.nodeIndex["node1"] = node

	mw := &metricsWriter{
		firstWrite: true,
		status:     http.StatusOK,
		bytes:      1024,
		firstAt:    time.Now(),
		lastAt:     time.Now(),
	}

	u := &usage{
		input:  100,
		output: 200,
	}

	// Should not panic without store
	srv.recordMetrics(context.Background(), "node1", time.Now(), mw, u, 0, 0, true, "claude")

	// Verify metrics were updated in memory
	if node.Metrics.Requests != 1 {
		t.Errorf("Requests = %d, want 1", node.Metrics.Requests)
	}
	if node.Metrics.TotalInputTokens != 100 {
		t.Errorf("TotalInputTokens = %d, want 100", node.Metrics.TotalInputTokens)
	}
	if node.Metrics.TotalOutputTokens != 200 {
		t.Errorf("TotalOutputTokens = %d, want 200", node.Metrics.TotalOutputTokens)
	}
}

func TestServer_recordMetrics_FailedRequest(t *testing.T) {
	srv := &Server{
		nodeIndex:   make(map[string]*Node),
		nodeAccount: make(map[string]*Account),
	}

	acc := &Account{
		ID:        "account1",
		FailedSet: make(map[string]struct{}),
	}

	node := &Node{
		ID:        "node1",
		Name:      "Test Node",
		AccountID: "account1",
		Metrics:   metrics{},
		Failed:    false,
	}
	srv.nodeIndex["node1"] = node
	srv.nodeAccount["node1"] = acc

	mw := &metricsWriter{
		firstWrite: true,
		status:     http.StatusInternalServerError,
		bytes:      512,
		firstAt:    time.Now(),
		lastAt:     time.Now(),
	}

	srv.recordMetrics(context.Background(), "node1", time.Now(), mw, nil, 0, 0, true, "claude")

	// Verify fail count was incremented
	if node.Metrics.FailCount != 1 {
		t.Errorf("FailCount = %d, want 1", node.Metrics.FailCount)
	}
}

func TestServer_recordMetrics_RecoveredNode(t *testing.T) {
	srv := &Server{
		nodeIndex:   make(map[string]*Node),
		nodeAccount: make(map[string]*Account),
	}

	acc := &Account{
		ID:        "account1",
		FailedSet: make(map[string]struct{}),
	}
	acc.FailedSet["node1"] = struct{}{}

	node := &Node{
		ID:        "node1",
		Name:      "Test Node",
		AccountID: "account1",
		Metrics: metrics{
			FailStreak: 3,
		},
		Failed:    true,
		LastError: "previous error",
	}
	srv.nodeIndex["node1"] = node
	srv.nodeAccount["node1"] = acc

	mw := &metricsWriter{
		firstWrite: true,
		status:     http.StatusOK,
		bytes:      1024,
		firstAt:    time.Now(),
		lastAt:     time.Now(),
	}

	srv.recordMetrics(context.Background(), "node1", time.Now(), mw, nil, 0, 0, true, "claude")

	// Verify node was recovered
	if node.Failed {
		t.Error("Failed should be false after successful request")
	}
	if node.Metrics.FailStreak != 0 {
		t.Errorf("FailStreak = %d, want 0", node.Metrics.FailStreak)
	}
	if node.LastError != "" {
		t.Errorf("LastError = %s, want empty", node.LastError)
	}
	if _, exists := acc.FailedSet["node1"]; exists {
		t.Error("node should be removed from FailedSet")
	}
}
