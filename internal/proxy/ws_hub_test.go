package proxy

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestNewWSHub(t *testing.T) {
	hub := NewWSHub()

	if hub == nil {
		t.Fatal("expected hub to be created")
	}
	if hub.clients == nil {
		t.Error("expected clients map to be initialized")
	}
	if hub.register == nil {
		t.Error("expected register channel to be initialized")
	}
	if hub.unregister == nil {
		t.Error("expected unregister channel to be initialized")
	}
	if hub.broadcast == nil {
		t.Error("expected broadcast channel to be initialized")
	}
}

func TestWSHubAddClient(t *testing.T) {
	hub := NewWSHub()

	client := &WSClient{
		hub:       hub,
		accountID: "test-account",
		send:      make(chan []byte, 256),
	}

	hub.addClient(client)

	hub.mu.RLock()
	defer hub.mu.RUnlock()

	if _, ok := hub.clients["test-account"]; !ok {
		t.Error("expected account to be in clients map")
	}
	if !hub.clients["test-account"][client] {
		t.Error("expected client to be registered")
	}
}

func TestWSHubAddNilClient(t *testing.T) {
	hub := NewWSHub()

	// Should not panic
	hub.addClient(nil)

	hub.mu.RLock()
	defer hub.mu.RUnlock()

	if len(hub.clients) != 0 {
		t.Error("expected no clients to be added")
	}
}

func TestWSHubRemoveClient(t *testing.T) {
	hub := NewWSHub()

	client := &WSClient{
		hub:       hub,
		accountID: "test-account",
		send:      make(chan []byte, 256),
	}

	hub.addClient(client)
	hub.removeClient(client)

	hub.mu.RLock()
	defer hub.mu.RUnlock()

	if _, ok := hub.clients["test-account"]; ok {
		t.Error("expected account to be removed from clients map")
	}
}

func TestWSHubRemoveNilClient(t *testing.T) {
	hub := NewWSHub()

	// Should not panic
	hub.removeClient(nil)
}

func TestWSHubRemoveNonExistentClient(t *testing.T) {
	hub := NewWSHub()

	client := &WSClient{
		hub:       hub,
		accountID: "test-account",
		send:      make(chan []byte, 256),
	}

	// Should not panic
	hub.removeClient(client)
}

func TestWSHubBroadcastToAccount(t *testing.T) {
	hub := NewWSHub()

	client := &WSClient{
		hub:       hub,
		accountID: "test-account",
		send:      make(chan []byte, 256),
	}

	hub.addClient(client)

	message := &WSMessage{
		AccountID: "test-account",
		Type:      "test_event",
		Payload:   map[string]string{"key": "value"},
	}

	hub.broadcastToAccount(message)

	select {
	case data := <-client.send:
		var received WSMessage
		if err := json.Unmarshal(data, &received); err != nil {
			t.Fatalf("failed to unmarshal message: %v", err)
		}
		if received.Type != "test_event" {
			t.Errorf("expected type 'test_event', got '%s'", received.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for message")
	}
}

func TestWSHubBroadcastToNonExistentAccount(t *testing.T) {
	hub := NewWSHub()

	message := &WSMessage{
		AccountID: "non-existent",
		Type:      "test_event",
		Payload:   map[string]string{"key": "value"},
	}

	// Should not panic
	hub.broadcastToAccount(message)
}

func TestWSHubBroadcastNilMessage(t *testing.T) {
	hub := NewWSHub()

	// Should not panic
	hub.broadcastToAccount(nil)
}

func TestWSHubBroadcast(t *testing.T) {
	hub := NewWSHub()

	client := &WSClient{
		hub:       hub,
		accountID: "test-account",
		send:      make(chan []byte, 256),
	}

	hub.addClient(client)

	// Start hub in background
	go hub.Run()

	// Give hub time to start
	time.Sleep(10 * time.Millisecond)

	hub.Broadcast("test-account", "test_event", map[string]string{"key": "value"})

	select {
	case data := <-client.send:
		var received WSMessage
		if err := json.Unmarshal(data, &received); err != nil {
			t.Fatalf("failed to unmarshal message: %v", err)
		}
		if received.Type != "test_event" {
			t.Errorf("expected type 'test_event', got '%s'", received.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for message")
	}
}

func TestWSHubBroadcastNilHub(t *testing.T) {
	var hub *WSHub

	// Should not panic
	hub.Broadcast("test-account", "test_event", nil)
}

func TestWSHubMultipleClients(t *testing.T) {
	hub := NewWSHub()

	client1 := &WSClient{
		hub:       hub,
		accountID: "account1",
		send:      make(chan []byte, 256),
	}

	client2 := &WSClient{
		hub:       hub,
		accountID: "account1",
		send:      make(chan []byte, 256),
	}

	client3 := &WSClient{
		hub:       hub,
		accountID: "account2",
		send:      make(chan []byte, 256),
	}

	hub.addClient(client1)
	hub.addClient(client2)
	hub.addClient(client3)

	message := &WSMessage{
		AccountID: "account1",
		Type:      "test_event",
		Payload:   map[string]string{"key": "value"},
	}

	hub.broadcastToAccount(message)

	// Both clients for account1 should receive the message
	receivedCount := 0
	timeout := time.After(100 * time.Millisecond)

	for receivedCount < 2 {
		select {
		case <-client1.send:
			receivedCount++
		case <-client2.send:
			receivedCount++
		case <-timeout:
			t.Errorf("expected 2 messages, got %d", receivedCount)
			return
		}
	}

	// client3 should not receive the message
	select {
	case <-client3.send:
		t.Error("client3 should not have received the message")
	case <-time.After(50 * time.Millisecond):
		// Expected
	}
}

func TestWSHubFullSendBuffer(t *testing.T) {
	hub := NewWSHub()

	// Create client with small buffer
	client := &WSClient{
		hub:       hub,
		accountID: "test-account",
		send:      make(chan []byte, 1),
	}

	hub.addClient(client)

	// Start hub in background
	go hub.Run()

	// Give hub time to start
	time.Sleep(10 * time.Millisecond)

	// Fill the buffer
	hub.Broadcast("test-account", "event1", nil)
	hub.Broadcast("test-account", "event2", nil)

	// Wait for unregister
	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	_, exists := hub.clients["test-account"]
	hub.mu.RUnlock()

	if exists {
		t.Error("expected client to be unregistered due to full buffer")
	}
}

func TestWSHubRegisterUnregisterViaChannels(t *testing.T) {
	hub := NewWSHub()

	client := &WSClient{
		hub:       hub,
		accountID: "test-account",
		send:      make(chan []byte, 256),
	}

	// Start hub in background
	go hub.Run()

	// Give hub time to start
	time.Sleep(10 * time.Millisecond)

	// Register via channel
	hub.register <- client

	// Wait for registration
	time.Sleep(10 * time.Millisecond)

	hub.mu.RLock()
	registered := hub.clients["test-account"][client]
	hub.mu.RUnlock()

	if !registered {
		t.Error("expected client to be registered")
	}

	// Unregister via channel
	hub.unregister <- client

	// Wait for unregistration
	time.Sleep(10 * time.Millisecond)

	hub.mu.RLock()
	_, exists := hub.clients["test-account"]
	hub.mu.RUnlock()

	if exists {
		t.Error("expected client to be unregistered")
	}
}

func TestWSHubInvalidJSONPayload(t *testing.T) {
	hub := NewWSHub()

	client := &WSClient{
		hub:       hub,
		accountID: "test-account",
		send:      make(chan []byte, 256),
	}

	hub.addClient(client)

	// Create message with payload that can't be marshaled
	message := &WSMessage{
		AccountID: "test-account",
		Type:      "test_event",
		Payload:   make(chan int), // channels can't be marshaled to JSON
	}

	// Should not panic, but message won't be sent
	hub.broadcastToAccount(message)

	select {
	case <-client.send:
		t.Error("should not have received message with invalid payload")
	case <-time.After(50 * time.Millisecond):
		// Expected
	}
}

func TestWSClientFields(t *testing.T) {
	hub := NewWSHub()
	conn := &websocket.Conn{} // Mock connection

	client := &WSClient{
		hub:       hub,
		conn:      conn,
		accountID: "test-account",
		send:      make(chan []byte, 256),
		isShare:   true,
	}

	if client.hub != hub {
		t.Error("expected hub to be set")
	}
	if client.conn != conn {
		t.Error("expected conn to be set")
	}
	if client.accountID != "test-account" {
		t.Error("expected accountID to be set")
	}
	if client.send == nil {
		t.Error("expected send channel to be initialized")
	}
	if !client.isShare {
		t.Error("expected isShare to be true")
	}
}

func TestWSMessageStructure(t *testing.T) {
	msg := WSMessage{
		AccountID: "test-account",
		Type:      "node_status",
		Payload:   map[string]interface{}{"status": "online"},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal message: %v", err)
	}

	var decoded WSMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal message: %v", err)
	}

	if decoded.AccountID != msg.AccountID {
		t.Errorf("expected AccountID %s, got %s", msg.AccountID, decoded.AccountID)
	}
	if decoded.Type != msg.Type {
		t.Errorf("expected Type %s, got %s", msg.Type, decoded.Type)
	}
}

func TestWSHubCleanupOnRemove(t *testing.T) {
	hub := NewWSHub()

	client1 := &WSClient{
		hub:       hub,
		accountID: "test-account",
		send:      make(chan []byte, 256),
	}

	client2 := &WSClient{
		hub:       hub,
		accountID: "test-account",
		send:      make(chan []byte, 256),
	}

	hub.addClient(client1)
	hub.addClient(client2)

	// Remove first client
	hub.removeClient(client1)

	hub.mu.RLock()
	accountClients := hub.clients["test-account"]
	hub.mu.RUnlock()

	if accountClients == nil {
		t.Error("expected account to still exist with remaining client")
	}
	if accountClients[client1] {
		t.Error("expected client1 to be removed")
	}
	if !accountClients[client2] {
		t.Error("expected client2 to still be registered")
	}

	// Remove second client
	hub.removeClient(client2)

	hub.mu.RLock()
	_, exists := hub.clients["test-account"]
	hub.mu.RUnlock()

	if exists {
		t.Error("expected account to be removed when last client is removed")
	}
}
