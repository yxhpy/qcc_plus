package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestCreateNotificationChannel(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Create test account
	acc := AccountRecord{
		ID:   "test-acc-notif",
		Name: "Notification Test Account",
	}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("create notification channel successfully", func(t *testing.T) {
		channel := NotificationChannelRecord{
			ID:          "channel-1",
			AccountID:   "test-acc-notif",
			ChannelType: "webhook",
			Name:        "Test Webhook",
			Config:      json.RawMessage(`{"url":"https://example.com/webhook"}`),
			Enabled:     true,
		}

		err := s.CreateNotificationChannel(ctx, channel)
		if err != nil {
			t.Fatalf("CreateNotificationChannel failed: %v", err)
		}

		// Verify creation
		retrieved, err := s.GetNotificationChannel(ctx, "channel-1")
		if err != nil {
			t.Fatalf("GetNotificationChannel failed: %v", err)
		}

		if retrieved.ID != channel.ID {
			t.Errorf("ID mismatch: got %s, want %s", retrieved.ID, channel.ID)
		}
		if retrieved.ChannelType != channel.ChannelType {
			t.Errorf("ChannelType mismatch: got %s, want %s", retrieved.ChannelType, channel.ChannelType)
		}
	})

	t.Run("create notification channel with empty ID fails", func(t *testing.T) {
		channel := NotificationChannelRecord{
			ID:          "",
			AccountID:   "test-acc-notif",
			ChannelType: "webhook",
		}

		err := s.CreateNotificationChannel(ctx, channel)
		if err == nil {
			t.Fatal("Expected error for empty ID, got nil")
		}
	})

	t.Run("create notification channel with empty account_id fails", func(t *testing.T) {
		channel := NotificationChannelRecord{
			ID:          "channel-2",
			AccountID:   "",
			ChannelType: "webhook",
		}

		err := s.CreateNotificationChannel(ctx, channel)
		if err == nil {
			t.Fatal("Expected error for empty account_id, got nil")
		}
	})

	t.Run("create notification channel with empty channel_type fails", func(t *testing.T) {
		channel := NotificationChannelRecord{
			ID:          "channel-3",
			AccountID:   "test-acc-notif",
			ChannelType: "",
		}

		err := s.CreateNotificationChannel(ctx, channel)
		if err == nil {
			t.Fatal("Expected error for empty channel_type, got nil")
		}
	})
}

func TestUpdateNotificationChannel(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Create test account
	acc := AccountRecord{
		ID:   "test-acc-update-notif",
		Name: "Update Notification Test",
	}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("update notification channel successfully", func(t *testing.T) {
		// Create channel
		channel := NotificationChannelRecord{
			ID:          "channel-update-1",
			AccountID:   "test-acc-update-notif",
			ChannelType: "webhook",
			Name:        "Original Name",
			Config:      json.RawMessage(`{"url":"https://example.com"}`),
			Enabled:     true,
		}
		if err := s.CreateNotificationChannel(ctx, channel); err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Update channel
		channel.Name = "Updated Name"
		channel.Config = json.RawMessage(`{"url":"https://updated.com"}`)
		channel.Enabled = false

		err := s.UpdateNotificationChannel(ctx, channel)
		if err != nil {
			t.Fatalf("UpdateNotificationChannel failed: %v", err)
		}

		// Verify update
		retrieved, err := s.GetNotificationChannel(ctx, "channel-update-1")
		if err != nil {
			t.Fatalf("GetNotificationChannel failed: %v", err)
		}

		if retrieved.Name != "Updated Name" {
			t.Errorf("Name not updated: got %s, want %s", retrieved.Name, "Updated Name")
		}
		if retrieved.Enabled {
			t.Error("Enabled should be false")
		}
	})

	t.Run("update notification channel with empty ID fails", func(t *testing.T) {
		channel := NotificationChannelRecord{
			ID:   "",
			Name: "Test",
		}

		err := s.UpdateNotificationChannel(ctx, channel)
		if err == nil {
			t.Fatal("Expected error for empty ID, got nil")
		}
	})

	t.Run("update non-existent notification channel returns ErrNotFound", func(t *testing.T) {
		channel := NotificationChannelRecord{
			ID:          "non-existent",
			AccountID:   "test-acc-update-notif",
			ChannelType: "webhook",
			Name:        "Test",
		}

		err := s.UpdateNotificationChannel(ctx, channel)
		if err != ErrNotFound {
			t.Errorf("Expected ErrNotFound, got %v", err)
		}
	})
}

func TestGetNotificationChannel(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Create test account
	acc := AccountRecord{
		ID:   "test-acc-get-notif",
		Name: "Get Notification Test",
	}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("get notification channel by valid ID", func(t *testing.T) {
		channel := NotificationChannelRecord{
			ID:          "channel-get-1",
			AccountID:   "test-acc-get-notif",
			ChannelType: "webhook",
			Name:        "Test Channel",
		}
		if err := s.CreateNotificationChannel(ctx, channel); err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		retrieved, err := s.GetNotificationChannel(ctx, "channel-get-1")
		if err != nil {
			t.Fatalf("GetNotificationChannel failed: %v", err)
		}

		if retrieved.ID != channel.ID {
			t.Errorf("ID mismatch: got %s, want %s", retrieved.ID, channel.ID)
		}
	})

	t.Run("get notification channel with empty ID fails", func(t *testing.T) {
		_, err := s.GetNotificationChannel(ctx, "")
		if err == nil {
			t.Fatal("Expected error for empty ID, got nil")
		}
	})

	t.Run("get non-existent notification channel returns ErrNotFound", func(t *testing.T) {
		_, err := s.GetNotificationChannel(ctx, "non-existent")
		if err != ErrNotFound {
			t.Errorf("Expected ErrNotFound, got %v", err)
		}
	})
}

func TestListNotificationChannels(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Create test accounts
	acc1 := AccountRecord{ID: "acc-list-notif-1", Name: "List Notif Account 1"}
	acc2 := AccountRecord{ID: "acc-list-notif-2", Name: "List Notif Account 2"}
	if err := s.CreateAccount(ctx, acc1); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	if err := s.CreateAccount(ctx, acc2); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("list notification channels by account", func(t *testing.T) {
		// Create channels
		channels := []NotificationChannelRecord{
			{ID: "ch-list-1", AccountID: "acc-list-notif-1", ChannelType: "webhook", Name: "Channel 1"},
			{ID: "ch-list-2", AccountID: "acc-list-notif-1", ChannelType: "email", Name: "Channel 2"},
			{ID: "ch-list-3", AccountID: "acc-list-notif-2", ChannelType: "webhook", Name: "Channel 3"},
		}
		for _, ch := range channels {
			if err := s.CreateNotificationChannel(ctx, ch); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}
		}

		// List channels for account 1
		retrieved, err := s.ListNotificationChannels(ctx, "acc-list-notif-1")
		if err != nil {
			t.Fatalf("ListNotificationChannels failed: %v", err)
		}

		if len(retrieved) != 2 {
			t.Errorf("Expected 2 channels, got %d", len(retrieved))
		}

		for _, ch := range retrieved {
			if ch.AccountID != "acc-list-notif-1" {
				t.Errorf("Expected account_id acc-list-notif-1, got %s", ch.AccountID)
			}
		}
	})

	t.Run("list notification channels for empty account", func(t *testing.T) {
		retrieved, err := s.ListNotificationChannels(ctx, "empty-account")
		if err != nil {
			t.Fatalf("ListNotificationChannels failed: %v", err)
		}

		if len(retrieved) != 0 {
			t.Errorf("Expected 0 channels, got %d", len(retrieved))
		}
	})
}

func TestNotificationSubscription(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Create test account and channel
	acc := AccountRecord{
		ID:   "test-acc-sub",
		Name: "Subscription Test Account",
	}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	channel := NotificationChannelRecord{
		ID:          "channel-sub-1",
		AccountID:   "test-acc-sub",
		ChannelType: "webhook",
		Name:        "Test Channel",
		Enabled:     true,
	}
	if err := s.CreateNotificationChannel(ctx, channel); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("upsert notification subscription successfully", func(t *testing.T) {
		sub := NotificationSubscriptionRecord{
			ID:        "sub-1",
			AccountID: "test-acc-sub",
			ChannelID: "channel-sub-1",
			EventType: "node_failure",
			Enabled:   true,
		}

		err := s.UpsertNotificationSubscription(ctx, sub)
		if err != nil {
			t.Fatalf("UpsertNotificationSubscription failed: %v", err)
		}

		// Verify creation
		retrieved, err := s.GetNotificationSubscription(ctx, "sub-1")
		if err != nil {
			t.Fatalf("GetNotificationSubscription failed: %v", err)
		}

		if retrieved.ID != sub.ID {
			t.Errorf("ID mismatch: got %s, want %s", retrieved.ID, sub.ID)
		}
		if retrieved.EventType != sub.EventType {
			t.Errorf("EventType mismatch: got %s, want %s", retrieved.EventType, sub.EventType)
		}
	})

	t.Run("upsert updates existing subscription", func(t *testing.T) {
		sub := NotificationSubscriptionRecord{
			ID:        "sub-1",
			AccountID: "test-acc-sub",
			ChannelID: "channel-sub-1",
			EventType: "node_failure",
			Enabled:   false, // Changed
		}

		err := s.UpsertNotificationSubscription(ctx, sub)
		if err != nil {
			t.Fatalf("UpsertNotificationSubscription failed: %v", err)
		}

		retrieved, err := s.GetNotificationSubscription(ctx, "sub-1")
		if err != nil {
			t.Fatalf("GetNotificationSubscription failed: %v", err)
		}

		if retrieved.Enabled {
			t.Error("Enabled should be false after update")
		}
	})

	t.Run("upsert with empty ID fails", func(t *testing.T) {
		sub := NotificationSubscriptionRecord{
			ID:        "",
			AccountID: "test-acc-sub",
			ChannelID: "channel-sub-1",
			EventType: "node_failure",
		}

		err := s.UpsertNotificationSubscription(ctx, sub)
		if err == nil {
			t.Fatal("Expected error for empty ID, got nil")
		}
	})

	t.Run("upsert with empty account_id fails", func(t *testing.T) {
		sub := NotificationSubscriptionRecord{
			ID:        "sub-2",
			AccountID: "",
			ChannelID: "channel-sub-1",
			EventType: "node_failure",
		}

		err := s.UpsertNotificationSubscription(ctx, sub)
		if err == nil {
			t.Fatal("Expected error for empty account_id, got nil")
		}
	})

	t.Run("get subscription with empty ID fails", func(t *testing.T) {
		_, err := s.GetNotificationSubscription(ctx, "")
		if err == nil {
			t.Fatal("Expected error for empty ID, got nil")
		}
	})

	t.Run("get non-existent subscription returns ErrNotFound", func(t *testing.T) {
		_, err := s.GetNotificationSubscription(ctx, "non-existent")
		if err != ErrNotFound {
			t.Errorf("Expected ErrNotFound, got %v", err)
		}
	})
}

func TestListNotificationSubscriptions(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Create test account and channels
	acc := AccountRecord{
		ID:   "test-acc-list-sub",
		Name: "List Subscription Test",
	}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	ch1 := NotificationChannelRecord{
		ID:          "ch-sub-1",
		AccountID:   "test-acc-list-sub",
		ChannelType: "webhook",
		Name:        "Channel 1",
		Enabled:     true,
	}
	ch2 := NotificationChannelRecord{
		ID:          "ch-sub-2",
		AccountID:   "test-acc-list-sub",
		ChannelType: "email",
		Name:        "Channel 2",
		Enabled:     true,
	}
	if err := s.CreateNotificationChannel(ctx, ch1); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	if err := s.CreateNotificationChannel(ctx, ch2); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Create subscriptions
	subs := []NotificationSubscriptionRecord{
		{ID: "sub-list-1", AccountID: "test-acc-list-sub", ChannelID: "ch-sub-1", EventType: "node_failure", Enabled: true},
		{ID: "sub-list-2", AccountID: "test-acc-list-sub", ChannelID: "ch-sub-1", EventType: "node_recovery", Enabled: true},
		{ID: "sub-list-3", AccountID: "test-acc-list-sub", ChannelID: "ch-sub-2", EventType: "node_failure", Enabled: true},
	}
	for _, sub := range subs {
		if err := s.UpsertNotificationSubscription(ctx, sub); err != nil {
			t.Fatalf("Setup failed: %v", err)
		}
	}

	t.Run("list all subscriptions for account", func(t *testing.T) {
		retrieved, err := s.ListNotificationSubscriptions(ctx, "test-acc-list-sub", "")
		if err != nil {
			t.Fatalf("ListNotificationSubscriptions failed: %v", err)
		}

		if len(retrieved) != 3 {
			t.Errorf("Expected 3 subscriptions, got %d", len(retrieved))
		}
	})

	t.Run("list subscriptions filtered by channel", func(t *testing.T) {
		retrieved, err := s.ListNotificationSubscriptions(ctx, "test-acc-list-sub", "ch-sub-1")
		if err != nil {
			t.Fatalf("ListNotificationSubscriptions failed: %v", err)
		}

		if len(retrieved) != 2 {
			t.Errorf("Expected 2 subscriptions, got %d", len(retrieved))
		}

		for _, sub := range retrieved {
			if sub.ChannelID != "ch-sub-1" {
				t.Errorf("Expected channel_id ch-sub-1, got %s", sub.ChannelID)
			}
		}
	})
}

func TestDeleteNotificationSubscription(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Create test account and channel
	acc := AccountRecord{
		ID:   "test-acc-del-sub",
		Name: "Delete Subscription Test",
	}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	channel := NotificationChannelRecord{
		ID:          "ch-del-sub",
		AccountID:   "test-acc-del-sub",
		ChannelType: "webhook",
		Name:        "Test Channel",
		Enabled:     true,
	}
	if err := s.CreateNotificationChannel(ctx, channel); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("delete subscription successfully", func(t *testing.T) {
		sub := NotificationSubscriptionRecord{
			ID:        "sub-del-1",
			AccountID: "test-acc-del-sub",
			ChannelID: "ch-del-sub",
			EventType: "node_failure",
			Enabled:   true,
		}
		if err := s.UpsertNotificationSubscription(ctx, sub); err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		err := s.DeleteNotificationSubscription(ctx, "sub-del-1")
		if err != nil {
			t.Fatalf("DeleteNotificationSubscription failed: %v", err)
		}

		// Verify deletion
		_, err = s.GetNotificationSubscription(ctx, "sub-del-1")
		if err != ErrNotFound {
			t.Errorf("Expected ErrNotFound after deletion, got %v", err)
		}
	})

	t.Run("delete subscription with empty ID fails", func(t *testing.T) {
		err := s.DeleteNotificationSubscription(ctx, "")
		if err == nil {
			t.Fatal("Expected error for empty ID, got nil")
		}
	})

	t.Run("delete non-existent subscription returns ErrNotFound", func(t *testing.T) {
		err := s.DeleteNotificationSubscription(ctx, "non-existent")
		if err != ErrNotFound {
			t.Errorf("Expected ErrNotFound, got %v", err)
		}
	})
}

func TestDeleteNotificationChannel(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Create test account
	acc := AccountRecord{
		ID:   "test-acc-del-ch",
		Name: "Delete Channel Test",
	}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("delete channel cascades to subscriptions", func(t *testing.T) {
		// Create channel
		channel := NotificationChannelRecord{
			ID:          "ch-del-1",
			AccountID:   "test-acc-del-ch",
			ChannelType: "webhook",
			Name:        "Test Channel",
			Enabled:     true,
		}
		if err := s.CreateNotificationChannel(ctx, channel); err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Create subscription
		sub := NotificationSubscriptionRecord{
			ID:        "sub-cascade-1",
			AccountID: "test-acc-del-ch",
			ChannelID: "ch-del-1",
			EventType: "node_failure",
			Enabled:   true,
		}
		if err := s.UpsertNotificationSubscription(ctx, sub); err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Delete channel
		err := s.DeleteNotificationChannel(ctx, "ch-del-1")
		if err != nil {
			t.Fatalf("DeleteNotificationChannel failed: %v", err)
		}

		// Verify channel deleted
		_, err = s.GetNotificationChannel(ctx, "ch-del-1")
		if err != ErrNotFound {
			t.Errorf("Expected ErrNotFound for channel, got %v", err)
		}

		// Verify subscription deleted
		_, err = s.GetNotificationSubscription(ctx, "sub-cascade-1")
		if err != ErrNotFound {
			t.Errorf("Expected ErrNotFound for subscription, got %v", err)
		}
	})

	t.Run("delete channel with empty ID fails", func(t *testing.T) {
		err := s.DeleteNotificationChannel(ctx, "")
		if err == nil {
			t.Fatal("Expected error for empty ID, got nil")
		}
	})

	t.Run("delete non-existent channel returns ErrNotFound", func(t *testing.T) {
		err := s.DeleteNotificationChannel(ctx, "non-existent")
		if err != ErrNotFound {
			t.Errorf("Expected ErrNotFound, got %v", err)
		}
	})
}

func TestListEnabledSubscriptionsForEvent(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Create test account
	acc := AccountRecord{
		ID:   "test-acc-enabled",
		Name: "Enabled Subscriptions Test",
	}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Create channels
	ch1 := NotificationChannelRecord{
		ID:          "ch-enabled-1",
		AccountID:   "test-acc-enabled",
		ChannelType: "webhook",
		Name:        "Enabled Channel",
		Enabled:     true,
	}
	ch2 := NotificationChannelRecord{
		ID:          "ch-disabled-1",
		AccountID:   "test-acc-enabled",
		ChannelType: "email",
		Name:        "Disabled Channel",
		Enabled:     false,
	}
	ch3 := NotificationChannelRecord{
		ID:          "ch-enabled-2",
		AccountID:   "test-acc-enabled",
		ChannelType: "webhook",
		Name:        "Enabled Channel 2",
		Enabled:     true,
	}
	if err := s.CreateNotificationChannel(ctx, ch1); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	if err := s.CreateNotificationChannel(ctx, ch2); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	if err := s.CreateNotificationChannel(ctx, ch3); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Create subscriptions
	subs := []NotificationSubscriptionRecord{
		{ID: "sub-en-1", AccountID: "test-acc-enabled", ChannelID: "ch-enabled-1", EventType: "node_failure", Enabled: true},
		{ID: "sub-en-2", AccountID: "test-acc-enabled", ChannelID: "ch-enabled-1", EventType: "node_recovery", Enabled: true},
		{ID: "sub-dis-1", AccountID: "test-acc-enabled", ChannelID: "ch-enabled-2", EventType: "node_failure", Enabled: false},
		{ID: "sub-dis-ch", AccountID: "test-acc-enabled", ChannelID: "ch-disabled-1", EventType: "node_failure", Enabled: true},
	}
	for _, sub := range subs {
		if err := s.UpsertNotificationSubscription(ctx, sub); err != nil {
			t.Fatalf("Setup failed: %v", err)
		}
	}

	t.Run("list enabled subscriptions for event", func(t *testing.T) {
		retrieved, err := s.ListEnabledSubscriptionsForEvent(ctx, "test-acc-enabled", "node_failure")
		if err != nil {
			t.Fatalf("ListEnabledSubscriptionsForEvent failed: %v", err)
		}

		// Should only return sub-en-1 (enabled subscription with enabled channel)
		if len(retrieved) != 1 {
			t.Errorf("Expected 1 enabled subscription, got %d", len(retrieved))
		}

		if len(retrieved) > 0 {
			if retrieved[0].Subscription.ID != "sub-en-1" {
				t.Errorf("Expected sub-en-1, got %s", retrieved[0].Subscription.ID)
			}
			if !retrieved[0].Channel.Enabled {
				t.Error("Channel should be enabled")
			}
		}
	})

	t.Run("list enabled subscriptions for different event", func(t *testing.T) {
		retrieved, err := s.ListEnabledSubscriptionsForEvent(ctx, "test-acc-enabled", "node_recovery")
		if err != nil {
			t.Fatalf("ListEnabledSubscriptionsForEvent failed: %v", err)
		}

		if len(retrieved) != 1 {
			t.Errorf("Expected 1 enabled subscription, got %d", len(retrieved))
		}
	})
}

func TestInsertNotificationHistory(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Create test account and channel
	acc := AccountRecord{
		ID:   "test-acc-history",
		Name: "History Test Account",
	}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	channel := NotificationChannelRecord{
		ID:          "ch-history-1",
		AccountID:   "test-acc-history",
		ChannelType: "webhook",
		Name:        "Test Channel",
		Enabled:     true,
	}
	if err := s.CreateNotificationChannel(ctx, channel); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("insert notification history successfully", func(t *testing.T) {
		now := time.Now()
		history := NotificationHistoryRecord{
			ID:        "history-1",
			AccountID: "test-acc-history",
			ChannelID: "ch-history-1",
			EventType: "node_failure",
			Title:     "Node Failed",
			Content:   "Node node-1 has failed",
			Status:    "sent",
			SentAt:    &now,
		}

		err := s.InsertNotificationHistory(ctx, history)
		if err != nil {
			t.Fatalf("InsertNotificationHistory failed: %v", err)
		}
	})

	t.Run("insert notification history with error", func(t *testing.T) {
		now := time.Now()
		history := NotificationHistoryRecord{
			ID:        "history-2",
			AccountID: "test-acc-history",
			ChannelID: "ch-history-1",
			EventType: "node_failure",
			Title:     "Node Failed",
			Content:   "Node node-2 has failed",
			Status:    "failed",
			Error:     "Connection timeout",
			SentAt:    &now,
		}

		err := s.InsertNotificationHistory(ctx, history)
		if err != nil {
			t.Fatalf("InsertNotificationHistory failed: %v", err)
		}
	})

	t.Run("insert notification history with empty ID fails", func(t *testing.T) {
		history := NotificationHistoryRecord{
			ID:        "",
			AccountID: "test-acc-history",
			ChannelID: "ch-history-1",
			EventType: "node_failure",
		}

		err := s.InsertNotificationHistory(ctx, history)
		if err == nil {
			t.Fatal("Expected error for empty ID, got nil")
		}
	})

	t.Run("insert notification history with empty account_id fails", func(t *testing.T) {
		history := NotificationHistoryRecord{
			ID:        "history-3",
			AccountID: "",
			ChannelID: "ch-history-1",
			EventType: "node_failure",
		}

		err := s.InsertNotificationHistory(ctx, history)
		if err == nil {
			t.Fatal("Expected error for empty account_id, got nil")
		}
	})
}
