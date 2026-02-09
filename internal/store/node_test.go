package store

import (
	"context"
	"testing"
	"time"
)

func TestUpsertNode(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Create test account first
	acc := AccountRecord{
		ID:   "test-acc",
		Name: "Test Account",
	}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("insert new node", func(t *testing.T) {
		node := NodeRecord{
			ID:        "node-1",
			AccountID: "test-acc",
			Name:      "Test Node",
			BaseURL:   "http://test.com",
			APIKey:    "test-key",
			Weight:    1,
		}

		err := s.UpsertNode(ctx, node)
		if err != nil {
			t.Fatalf("UpsertNode failed: %v", err)
		}

		// Verify node was created
		nodes, err := s.GetNodesByAccount(ctx, "test-acc")
		if err != nil {
			t.Fatalf("GetNodesByAccount failed: %v", err)
		}

		if len(nodes) != 1 {
			t.Fatalf("Expected 1 node, got %d", len(nodes))
		}

		if nodes[0].Name != node.Name {
			t.Errorf("Name mismatch: got %s, want %s", nodes[0].Name, node.Name)
		}
		if nodes[0].BaseURL != node.BaseURL {
			t.Errorf("BaseURL mismatch: got %s, want %s", nodes[0].BaseURL, node.BaseURL)
		}
	})

	t.Run("update existing node", func(t *testing.T) {
		// Insert initial node
		node := NodeRecord{
			ID:        "node-2",
			AccountID: "test-acc",
			Name:      "Original Name",
			BaseURL:   "http://original.com",
			APIKey:    "original-key",
			Weight:    1,
		}

		if err := s.UpsertNode(ctx, node); err != nil {
			t.Fatalf("Initial UpsertNode failed: %v", err)
		}

		// Update the node
		updated := NodeRecord{
			ID:        "node-2",
			AccountID: "test-acc",
			Name:      "Updated Name",
			BaseURL:   "http://updated.com",
			APIKey:    "updated-key",
			Weight:    2,
		}

		if err := s.UpsertNode(ctx, updated); err != nil {
			t.Fatalf("Update UpsertNode failed: %v", err)
		}

		// Verify update
		nodes, err := s.GetNodesByAccount(ctx, "test-acc")
		if err != nil {
			t.Fatalf("GetNodesByAccount failed: %v", err)
		}

		var found *NodeRecord
		for i := range nodes {
			if nodes[i].ID == "node-2" {
				found = &nodes[i]
				break
			}
		}

		if found == nil {
			t.Fatal("Updated node not found")
		}

		if found.Name != updated.Name {
			t.Errorf("Name not updated: got %s, want %s", found.Name, updated.Name)
		}
		if found.BaseURL != updated.BaseURL {
			t.Errorf("BaseURL not updated: got %s, want %s", found.BaseURL, updated.BaseURL)
		}
		if found.Weight != updated.Weight {
			t.Errorf("Weight not updated: got %d, want %d", found.Weight, updated.Weight)
		}
	})

	t.Run("default health check method", func(t *testing.T) {
		node := NodeRecord{
			ID:        "node-3",
			AccountID: "test-acc",
			Name:      "Default Health Check",
			BaseURL:   "http://test.com",
			APIKey:    "test-key",
			// HealthCheckMethod not set
		}

		if err := s.UpsertNode(ctx, node); err != nil {
			t.Fatalf("UpsertNode failed: %v", err)
		}

		nodes, err := s.GetNodesByAccount(ctx, "test-acc")
		if err != nil {
			t.Fatalf("GetNodesByAccount failed: %v", err)
		}

		var found *NodeRecord
		for i := range nodes {
			if nodes[i].ID == "node-3" {
				found = &nodes[i]
				break
			}
		}

		if found == nil {
			t.Fatal("Node not found")
		}

		if found.HealthCheckMethod != "api" {
			t.Errorf("Expected default health check method 'api', got %s", found.HealthCheckMethod)
		}
	})

	t.Run("auto timestamps", func(t *testing.T) {
		node := NodeRecord{
			ID:        "node-4",
			AccountID: "test-acc",
			Name:      "Timestamp Test",
			BaseURL:   "http://test.com",
			APIKey:    "test-key",
		}

		before := time.Now()
		if err := s.UpsertNode(ctx, node); err != nil {
			t.Fatalf("UpsertNode failed: %v", err)
		}
		after := time.Now()

		nodes, err := s.GetNodesByAccount(ctx, "test-acc")
		if err != nil {
			t.Fatalf("GetNodesByAccount failed: %v", err)
		}

		var found *NodeRecord
		for i := range nodes {
			if nodes[i].ID == "node-4" {
				found = &nodes[i]
				break
			}
		}

		if found == nil {
			t.Fatal("Node not found")
		}

		if found.CreatedAt.Before(before) || found.CreatedAt.After(after) {
			t.Errorf("CreatedAt not in expected range: %v", found.CreatedAt)
		}
	})
}

func TestGetNodesByAccount(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Create test accounts
	acc1 := AccountRecord{ID: "acc-1", Name: "Account 1"}
	acc2 := AccountRecord{ID: "acc-2", Name: "Account 2"}
	if err := s.CreateAccount(ctx, acc1); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	if err := s.CreateAccount(ctx, acc2); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("get nodes for account with no nodes", func(t *testing.T) {
		nodes, err := s.GetNodesByAccount(ctx, "acc-1")
		if err != nil {
			t.Fatalf("GetNodesByAccount failed: %v", err)
		}

		if len(nodes) != 0 {
			t.Errorf("Expected 0 nodes, got %d", len(nodes))
		}
	})

	t.Run("get nodes for account with multiple nodes", func(t *testing.T) {
		// Create nodes for acc-1
		nodes := []NodeRecord{
			{ID: "node-1", AccountID: "acc-1", Name: "Node 1", BaseURL: "http://1.com", APIKey: "key1", Weight: 2},
			{ID: "node-2", AccountID: "acc-1", Name: "Node 2", BaseURL: "http://2.com", APIKey: "key2", Weight: 1},
			{ID: "node-3", AccountID: "acc-1", Name: "Node 3", BaseURL: "http://3.com", APIKey: "key3", Weight: 3},
		}

		for _, node := range nodes {
			if err := s.UpsertNode(ctx, node); err != nil {
				t.Fatalf("UpsertNode failed: %v", err)
			}
		}

		// Create node for acc-2 (should not be returned)
		node4 := NodeRecord{ID: "node-4", AccountID: "acc-2", Name: "Node 4", BaseURL: "http://4.com", APIKey: "key4", Weight: 1}
		if err := s.UpsertNode(ctx, node4); err != nil {
			t.Fatalf("UpsertNode failed: %v", err)
		}

		// Get nodes for acc-1
		retrieved, err := s.GetNodesByAccount(ctx, "acc-1")
		if err != nil {
			t.Fatalf("GetNodesByAccount failed: %v", err)
		}

		if len(retrieved) != 3 {
			t.Errorf("Expected 3 nodes, got %d", len(retrieved))
		}

		// Verify order (should be by weight ASC, then created_at ASC)
		if retrieved[0].Weight != 1 {
			t.Errorf("First node should have weight 1, got %d", retrieved[0].Weight)
		}
		if retrieved[1].Weight != 2 {
			t.Errorf("Second node should have weight 2, got %d", retrieved[1].Weight)
		}
		if retrieved[2].Weight != 3 {
			t.Errorf("Third node should have weight 3, got %d", retrieved[2].Weight)
		}
	})

	t.Run("nodes are isolated by account", func(t *testing.T) {
		// Get nodes for acc-2
		nodes, err := s.GetNodesByAccount(ctx, "acc-2")
		if err != nil {
			t.Fatalf("GetNodesByAccount failed: %v", err)
		}

		if len(nodes) != 1 {
			t.Errorf("Expected 1 node for acc-2, got %d", len(nodes))
		}

		if nodes[0].ID != "node-4" {
			t.Errorf("Expected node-4, got %s", nodes[0].ID)
		}
	})
}

func TestDeleteNode(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Create test account
	acc := AccountRecord{ID: "test-acc", Name: "Test Account"}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("delete existing node", func(t *testing.T) {
		// Create node
		node := NodeRecord{
			ID:        "node-delete",
			AccountID: "test-acc",
			Name:      "Delete Test",
			BaseURL:   "http://test.com",
			APIKey:    "test-key",
		}

		if err := s.UpsertNode(ctx, node); err != nil {
			t.Fatalf("UpsertNode failed: %v", err)
		}

		// Delete node
		if err := s.DeleteNode(ctx, "node-delete"); err != nil {
			t.Fatalf("DeleteNode failed: %v", err)
		}

		// Verify deletion
		nodes, err := s.GetNodesByAccount(ctx, "test-acc")
		if err != nil {
			t.Fatalf("GetNodesByAccount failed: %v", err)
		}

		for _, n := range nodes {
			if n.ID == "node-delete" {
				t.Error("Node should have been deleted")
			}
		}
	})

	t.Run("delete non-existent node does not error", func(t *testing.T) {
		err := s.DeleteNode(ctx, "non-existent")
		if err != nil {
			t.Errorf("DeleteNode should not error for non-existent node, got %v", err)
		}
	})
}
