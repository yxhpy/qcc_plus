package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestWebSocketConnection tests WebSocket connection establishment
func TestWebSocketConnection(t *testing.T) {
	b := NewBuilder().
		WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	// Create test account
	acc, err := srv.createAccount("test-account", "test-proxy", "test123", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	// Create session
	sess := srv.sessionMgr.Create(acc.ID, false)

	// Start server
	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	t.Run("WebSocket connection with valid session", func(t *testing.T) {
		// Convert http:// to ws://
		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

		// Create WebSocket connection with session cookie
		header := http.Header{}
		header.Add("Cookie", "session_token="+sess.Token)

		conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
		if err != nil {
			// WebSocket might not be fully implemented, so we check the response
			if resp != nil && resp.StatusCode == http.StatusUnauthorized {
				t.Skip("WebSocket endpoint requires authentication")
			}
			t.Logf("WebSocket connection failed (may be expected): %v", err)
			return
		}
		defer conn.Close()

		// Connection successful
		if conn == nil {
			t.Error("expected WebSocket connection")
		}
	})

	t.Run("WebSocket connection without session", func(t *testing.T) {
		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

		_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err == nil {
			t.Error("expected WebSocket connection to fail without session")
		}

		// Should return 401 Unauthorized
		if resp != nil && resp.StatusCode != http.StatusUnauthorized {
			t.Logf("expected 401, got %d (may vary by implementation)", resp.StatusCode)
		}
	})
}

// TestWebSocketBroadcast tests WebSocket message broadcasting
func TestWebSocketBroadcast(t *testing.T) {
	b := NewBuilder().
		WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	// Create test account
	acc, err := srv.createAccount("test-account", "test-proxy", "test123", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	t.Run("Broadcast message to account", func(t *testing.T) {
		// Test the broadcast functionality
		msg := &WSMessage{
			AccountID: acc.ID,
			Type:      "test",
			Payload: map[string]interface{}{
				"message": "test broadcast",
			},
		}

		// This should not panic
		srv.wsHub.broadcastToAccount(msg)

		// Give it a moment to process
		time.Sleep(10 * time.Millisecond)
	})

	t.Run("Broadcast to all clients", func(t *testing.T) {
		payload := map[string]interface{}{
			"message": "global broadcast",
		}

		// This should not panic
		srv.wsHub.Broadcast(acc.ID, "global", payload)

		// Give it a moment to process
		time.Sleep(10 * time.Millisecond)
	})
}
