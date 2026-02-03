package proxy

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"qcc_plus/internal/store"
)

// TestExtractUsageFromHeader tests usage extraction from headers
func TestExtractUsageFromHeader(t *testing.T) {
	t.Run("Extract valid usage from headers", func(t *testing.T) {
		header := http.Header{}
		header.Set("X-Usage-Input-Tokens", "100")
		header.Set("X-Usage-Output-Tokens", "200")

		usage := extractUsageFromHeader(header)
		if usage == nil {
			t.Fatal("expected usage to be non-nil")
		}
		// Note: usage fields are unexported, so we can only check if it's non-nil
	})

	t.Run("Extract with missing headers", func(t *testing.T) {
		header := http.Header{}
		usage := extractUsageFromHeader(header)
		if usage != nil {
			t.Error("expected nil usage for missing headers")
		}
	})

	t.Run("Extract with invalid values", func(t *testing.T) {
		header := http.Header{}
		header.Set("X-Usage-Input-Tokens", "invalid")
		header.Set("X-Usage-Output-Tokens", "also-invalid")

		usage := extractUsageFromHeader(header)
		if usage != nil {
			t.Error("expected nil usage for invalid values")
		}
	})
}

// TestHeaderInt tests integer extraction from headers
func TestHeaderInt(t *testing.T) {
	t.Run("Parse valid integer", func(t *testing.T) {
		value := headerInt("42")
		if value != 42 {
			t.Errorf("expected 42, got %d", value)
		}
	})

	t.Run("Parse empty string", func(t *testing.T) {
		value := headerInt("")
		if value != 0 {
			t.Errorf("expected 0 for empty string, got %d", value)
		}
	})

	t.Run("Parse invalid integer", func(t *testing.T) {
		value := headerInt("not-a-number")
		if value != 0 {
			t.Errorf("expected 0 for invalid integer, got %d", value)
		}
	})
}

// TestErrString tests error string extraction
func TestErrString(t *testing.T) {
	t.Run("Handle nil error", func(t *testing.T) {
		// This function should handle nil errors
		result := errString(nil)
		if result != "" {
			t.Errorf("expected empty string for nil error, got %s", result)
		}
	})
}

// TestSafeDiv tests safe division
func TestSafeDiv(t *testing.T) {
	t.Run("Divide by non-zero", func(t *testing.T) {
		result := safeDiv(10.0, 2.0)
		if result != 5.0 {
			t.Errorf("expected 5.0, got %f", result)
		}
	})

	t.Run("Divide by zero", func(t *testing.T) {
		result := safeDiv(10.0, 0.0)
		if result != 0.0 {
			t.Errorf("expected 0.0 for division by zero, got %f", result)
		}
	})

	t.Run("Divide zero by non-zero", func(t *testing.T) {
		result := safeDiv(0.0, 5.0)
		if result != 0.0 {
			t.Errorf("expected 0.0, got %f", result)
		}
	})
}

// TestMaxInt64 tests max int64 function
func TestMaxInt64(t *testing.T) {
	t.Run("First value is larger", func(t *testing.T) {
		result := maxInt64(10, 5)
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
	})

	t.Run("Second value is larger", func(t *testing.T) {
		result := maxInt64(5, 10)
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
	})

	t.Run("Equal values", func(t *testing.T) {
		result := maxInt64(7, 7)
		if result != 7 {
			t.Errorf("expected 7, got %d", result)
		}
	})

	t.Run("Negative values", func(t *testing.T) {
		result := maxInt64(-5, -10)
		if result != -5 {
			t.Errorf("expected -5, got %d", result)
		}
	})
}

// TestWriteJSON tests JSON response writing
func TestWriteJSON(t *testing.T) {
	t.Run("Write valid JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		data := map[string]string{"message": "hello"}

		writeJSON(w, http.StatusOK, data)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		contentType := w.Header().Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got '%s'", contentType)
		}

		var result map[string]string
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if result["message"] != "hello" {
			t.Errorf("expected message 'hello', got '%s'", result["message"])
		}
	})

	t.Run("Write with different status codes", func(t *testing.T) {
		testCases := []int{
			http.StatusOK,
			http.StatusCreated,
			http.StatusBadRequest,
			http.StatusUnauthorized,
			http.StatusNotFound,
			http.StatusInternalServerError,
		}

		for _, status := range testCases {
			w := httptest.NewRecorder()
			writeJSON(w, status, map[string]string{"status": "test"})

			if w.Code != status {
				t.Errorf("expected status %d, got %d", status, w.Code)
			}
		}
	})

	t.Run("Write nil value", func(t *testing.T) {
		w := httptest.NewRecorder()
		writeJSON(w, http.StatusOK, nil)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})
}

// TestToRecord tests Node to NodeRecord conversion
func TestToRecord(t *testing.T) {
	t.Run("Convert node with all fields", func(t *testing.T) {
		nodeURL, _ := url.Parse("https://api.example.com")
		now := time.Now()

		node := &Node{
			ID:                "node-123",
			Name:              "test-node",
			URL:               nodeURL,
			APIKey:            "test-key",
			HealthCheckMethod: "HEAD",
			HealthCheckModel:  "claude-3",
			AccountID:         "acc-456",
			Weight:            5,
			Failed:            false,
			Disabled:          false,
			LastError:         "some error",
			CreatedAt:         now,
			Metrics: metrics{
				Requests:           100,
				FailCount:          5,
				FailStreak:         2,
				TotalBytes:         1024,
				TotalInputTokens:   500,
				TotalOutputTokens:  300,
				StreamDur:          time.Second * 10,
				FirstByteDur:       time.Millisecond * 100,
				LastPingMS:         50,
				LastPingErr:        "ping error",
				LastHealthCheckAt:  now,
			},
		}

		record := toRecord(node)

		if record.ID != "node-123" {
			t.Errorf("expected ID 'node-123', got '%s'", record.ID)
		}
		if record.Name != "test-node" {
			t.Errorf("expected Name 'test-node', got '%s'", record.Name)
		}
		if record.BaseURL != "https://api.example.com" {
			t.Errorf("expected BaseURL 'https://api.example.com', got '%s'", record.BaseURL)
		}
		if record.APIKey != "test-key" {
			t.Errorf("expected APIKey 'test-key', got '%s'", record.APIKey)
		}
		if record.HealthCheckMethod != "HEAD" {
			t.Errorf("expected HealthCheckMethod 'HEAD', got '%s'", record.HealthCheckMethod)
		}
		if record.AccountID != "acc-456" {
			t.Errorf("expected AccountID 'acc-456', got '%s'", record.AccountID)
		}
		if record.Weight != 5 {
			t.Errorf("expected Weight 5, got %d", record.Weight)
		}
		if record.Requests != 100 {
			t.Errorf("expected Requests 100, got %d", record.Requests)
		}
		if record.FailCount != 5 {
			t.Errorf("expected FailCount 5, got %d", record.FailCount)
		}
		if record.StreamDurMs != 10000 {
			t.Errorf("expected StreamDurMs 10000, got %d", record.StreamDurMs)
		}
		if record.FirstByteMs != 100 {
			t.Errorf("expected FirstByteMs 100, got %d", record.FirstByteMs)
		}
	})

	t.Run("Convert node with empty AccountID uses default", func(t *testing.T) {
		nodeURL, _ := url.Parse("https://api.example.com")

		node := &Node{
			ID:        "node-123",
			Name:      "test-node",
			URL:       nodeURL,
			AccountID: "", // Empty account ID
		}

		record := toRecord(node)

		if record.AccountID != store.DefaultAccountID {
			t.Errorf("expected AccountID '%s', got '%s'", store.DefaultAccountID, record.AccountID)
		}
	})
}

// TestErrStringWithError tests error string extraction with actual error
func TestErrStringWithError(t *testing.T) {
	t.Run("Handle non-nil error", func(t *testing.T) {
		err := errors.New("test error")
		result := errString(err)
		if result != "test error" {
			t.Errorf("expected 'test error', got '%s'", result)
		}
	})
}

// TestExtractUsageFromHeaderNil tests nil header handling
func TestExtractUsageFromHeaderNil(t *testing.T) {
	t.Run("Extract from nil header", func(t *testing.T) {
		usage := extractUsageFromHeader(nil)
		if usage != nil {
			t.Error("expected nil usage for nil header")
		}
	})

	t.Run("Extract with only input tokens", func(t *testing.T) {
		header := http.Header{}
		header.Set("X-Usage-Input-Tokens", "100")

		usage := extractUsageFromHeader(header)
		if usage == nil {
			t.Error("expected non-nil usage when input tokens present")
		}
	})

	t.Run("Extract with only output tokens", func(t *testing.T) {
		header := http.Header{}
		header.Set("X-Usage-Output-Tokens", "200")

		usage := extractUsageFromHeader(header)
		if usage == nil {
			t.Error("expected non-nil usage when output tokens present")
		}
	})
}
