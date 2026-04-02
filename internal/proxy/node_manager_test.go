package proxy

import (
	"testing"
)

// TestAddNode tests the addNode function
func TestAddNode(t *testing.T) {
	b := NewBuilder().
		WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	// Create test account
	acc, err := srv.createAccount("test-account", "test-proxy", "test123", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	t.Run("add node successfully", func(t *testing.T) {
		node, err := srv.addNodeToAccount(acc, "test-node", "http://test.example.com", "api-key", 1)
		if err != nil {
			t.Fatalf("addNode failed: %v", err)
		}
		if node == nil {
			t.Fatal("expected node, got nil")
		}
		if node.Name != "test-node" {
			t.Errorf("expected name 'test-node', got %s", node.Name)
		}
		if node.Weight != 1 {
			t.Errorf("expected weight 1, got %d", node.Weight)
		}
	})

	t.Run("add node with empty URL fails", func(t *testing.T) {
		_, err := srv.addNodeToAccount(acc, "test", "", "key", 1)
		if err == nil {
			t.Fatal("expected error for empty URL")
		}
	})

	t.Run("add node with invalid URL fails", func(t *testing.T) {
		_, err := srv.addNodeToAccount(acc, "test", "://invalid", "key", 1)
		if err == nil {
			t.Fatal("expected error for invalid URL")
		}
	})

	t.Run("add node with zero weight defaults to 1", func(t *testing.T) {
		node, err := srv.addNodeToAccount(acc, "test-zero", "http://test.example.com", "key", 0)
		if err != nil {
			t.Fatalf("addNode failed: %v", err)
		}
		if node.Weight != 1 {
			t.Errorf("expected weight 1, got %d", node.Weight)
		}
	})

	t.Run("add node with empty name uses host", func(t *testing.T) {
		node, err := srv.addNodeToAccount(acc, "", "http://auto-name.com", "key", 1)
		if err != nil {
			t.Fatalf("addNode failed: %v", err)
		}
		if node.Name != "auto-name.com" {
			t.Errorf("expected name 'auto-name.com', got %s", node.Name)
		}
	})

	t.Run("openai protocol forces api health method", func(t *testing.T) {
		node, err := srv.addNodeWithMethod(acc, "openai-node", "http://openai.example.com", "key", 1, HealthCheckMethodCLI, "", nil, SourceProtocolOpenAI, "", "")
		if err != nil {
			t.Fatalf("addNodeWithMethod failed: %v", err)
		}
		if node.HealthCheckMethod != HealthCheckMethodAPI {
			t.Fatalf("expected health method %s for openai protocol, got %s", HealthCheckMethodAPI, node.HealthCheckMethod)
		}
	})

	t.Run("gemini protocol forces api health method", func(t *testing.T) {
		node, err := srv.addNodeWithMethod(acc, "gemini-node", "http://gemini.example.com", "key", 1, HealthCheckMethodHEAD, "", nil, SourceProtocolGemini, "", "")
		if err != nil {
			t.Fatalf("addNodeWithMethod failed: %v", err)
		}
		if node.HealthCheckMethod != HealthCheckMethodAPI {
			t.Fatalf("expected health method %s for gemini protocol, got %s", HealthCheckMethodAPI, node.HealthCheckMethod)
		}
	})
}

// TestUpdateNode tests the updateNode function
func TestUpdateNode(t *testing.T) {
	b := NewBuilder().
		WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	// Create test account
	acc, err := srv.createAccount("test-account", "test-proxy", "test123", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	// Add a node first
	node, err := srv.addNodeToAccount(acc, "original", "http://original.com", "original-key", 2)
	if err != nil {
		t.Fatalf("addNode failed: %v", err)
	}

	t.Run("update node successfully", func(t *testing.T) {
		newAPIKey := "new-key"
		err := srv.updateNode(node.ID, "updated", "http://updated.com", &newAPIKey, 3, nil, nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("updateNode failed: %v", err)
		}

		updated := srv.getNode(node.ID)
		if updated.Name != "updated" {
			t.Errorf("expected name 'updated', got %s", updated.Name)
		}
		if updated.URL.String() != "http://updated.com" {
			t.Errorf("expected URL 'http://updated.com', got %s", updated.URL.String())
		}
		if updated.APIKey != "new-key" {
			t.Errorf("expected APIKey 'new-key', got %s", updated.APIKey)
		}
		if updated.Weight != 3 {
			t.Errorf("expected weight 3, got %d", updated.Weight)
		}
	})

	t.Run("update non-existent node fails", func(t *testing.T) {
		err := srv.updateNode("non-existent", "test", "http://test.com", nil, 1, nil, nil, nil, nil, nil, nil)
		if err == nil {
			t.Fatal("expected error for non-existent node")
		}
	})

	t.Run("update node with empty URL fails", func(t *testing.T) {
		err := srv.updateNode(node.ID, "test", "", nil, 1, nil, nil, nil, nil, nil, nil)
		if err == nil {
			t.Fatal("expected error for empty URL")
		}
	})

	t.Run("update node with invalid URL fails", func(t *testing.T) {
		err := srv.updateNode(node.ID, "test", "://invalid", nil, 1, nil, nil, nil, nil, nil, nil)
		if err == nil {
			t.Fatal("expected error for invalid URL")
		}
	})

	t.Run("update node preserves API key when nil", func(t *testing.T) {
		originalKey := srv.getNode(node.ID).APIKey
		err := srv.updateNode(node.ID, "test", "http://test.com", nil, 1, nil, nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("updateNode failed: %v", err)
		}
		if srv.getNode(node.ID).APIKey != originalKey {
			t.Error("API key should be preserved when nil")
		}
	})

	t.Run("update node with zero weight defaults to 1", func(t *testing.T) {
		err := srv.updateNode(node.ID, "test", "http://test.com", nil, 0, nil, nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("updateNode failed: %v", err)
		}
		if srv.getNode(node.ID).Weight != 1 {
			t.Errorf("expected weight 1, got %d", srv.getNode(node.ID).Weight)
		}
	})

	t.Run("update node protocol to openai forces api health method", func(t *testing.T) {
		method := HealthCheckMethodCLI
		proto := SourceProtocolOpenAI
		err := srv.updateNode(node.ID, "test", "http://test.com", nil, 1, &method, nil, nil, &proto, nil, nil)
		if err != nil {
			t.Fatalf("updateNode failed: %v", err)
		}
		if got := srv.getNode(node.ID).HealthCheckMethod; got != HealthCheckMethodAPI {
			t.Fatalf("expected health method %s after protocol update, got %s", HealthCheckMethodAPI, got)
		}
	})
}

// TestDeleteNode tests the deleteNode function
func TestDeleteNode(t *testing.T) {
	b := NewBuilder().
		WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	// Create test account
	acc, err := srv.createAccount("test-account", "test-proxy", "test123", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	t.Run("delete node successfully", func(t *testing.T) {
		node, err := srv.addNodeToAccount(acc, "to-delete", "http://delete.com", "key", 1)
		if err != nil {
			t.Fatalf("addNode failed: %v", err)
		}

		err = srv.deleteNode(node.ID)
		if err != nil {
			t.Fatalf("deleteNode failed: %v", err)
		}

		// Verify node is deleted
		if srv.getNode(node.ID) != nil {
			t.Error("node should be deleted")
		}
	})

	t.Run("delete non-existent node fails", func(t *testing.T) {
		err := srv.deleteNode("non-existent")
		if err == nil {
			t.Fatal("expected error for non-existent node")
		}
	})

	t.Run("delete active node clears active ID", func(t *testing.T) {
		node, err := srv.addNodeToAccount(acc, "active-delete", "http://active.com", "key", 1)
		if err != nil {
			t.Fatalf("addNode failed: %v", err)
		}

		// Make it active
		acc.ActiveID = node.ID

		err = srv.deleteNode(node.ID)
		if err != nil {
			t.Fatalf("deleteNode failed: %v", err)
		}

		if acc.ActiveID != "" {
			t.Error("active ID should be cleared after deleting active node")
		}
	})
}

// TestGetNode tests the getNode function
func TestGetNode(t *testing.T) {
	b := NewBuilder().
		WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	// Create test account
	acc, err := srv.createAccount("test-account", "test-proxy", "test123", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	t.Run("get existing node", func(t *testing.T) {
		node, err := srv.addNodeToAccount(acc, "test-get", "http://get.com", "key", 1)
		if err != nil {
			t.Fatalf("addNode failed: %v", err)
		}

		retrieved := srv.getNode(node.ID)
		if retrieved == nil {
			t.Fatal("expected node, got nil")
		}
		if retrieved.ID != node.ID {
			t.Errorf("expected ID %s, got %s", node.ID, retrieved.ID)
		}
	})

	t.Run("get non-existent node returns nil", func(t *testing.T) {
		node := srv.getNode("non-existent")
		if node != nil {
			t.Error("expected nil for non-existent node")
		}
	})
}

// TestSelectBestAndActivateExcluding tests the selectBestAndActivateExcluding function
func TestSelectBestAndActivateExcluding(t *testing.T) {
	b := NewBuilder().
		WithUpstream("http://example.com")
	srv := buildServerNoWarmup(t, b)

	// Create test account
	acc, err := srv.createAccount("test-account", "test-proxy", "test123", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	// Add multiple nodes with different weights
	node1, _ := srv.addNodeToAccount(acc, "node1", "http://node1.com", "key1", 1)
	node2, _ := srv.addNodeToAccount(acc, "node2", "http://node2.com", "key2", 2)
	node3, _ := srv.addNodeToAccount(acc, "node3", "http://node3.com", "key3", 3)

	t.Run("select best excluding one node", func(t *testing.T) {
		skipNodes := map[string]bool{node1.ID: true}
		selected, err := srv.selectBestAndActivateExcluding(acc, skipNodes, "test")
		if err != nil {
			t.Fatalf("selectBestAndActivateExcluding failed: %v", err)
		}
		if selected.ID == node1.ID {
			t.Error("should not select excluded node")
		}
		// Should select node2 (weight 2) since node1 (weight 1) is excluded
		if selected.ID != node2.ID {
			t.Errorf("expected node2, got %s", selected.Name)
		}
	})

	t.Run("select best excluding multiple nodes", func(t *testing.T) {
		skipNodes := map[string]bool{node1.ID: true, node2.ID: true}
		selected, err := srv.selectBestAndActivateExcluding(acc, skipNodes, "test")
		if err != nil {
			t.Fatalf("selectBestAndActivateExcluding failed: %v", err)
		}
		if selected.ID != node3.ID {
			t.Errorf("expected node3, got %s", selected.Name)
		}
	})

	t.Run("select best excluding all nodes fails", func(t *testing.T) {
		skipNodes := map[string]bool{node1.ID: true, node2.ID: true, node3.ID: true}
		_, err := srv.selectBestAndActivateExcluding(acc, skipNodes, "test")
		if err == nil {
			t.Fatal("expected error when all nodes excluded")
		}
	})
}
