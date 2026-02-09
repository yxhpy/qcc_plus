package notify

import (
	"strings"
	"testing"
)

func TestDDLNotificationChannels(t *testing.T) {
	if DDLNotificationChannels == "" {
		t.Error("DDLNotificationChannels should not be empty")
	}

	// Check for required table name
	if !strings.Contains(DDLNotificationChannels, "notification_channels") {
		t.Error("DDLNotificationChannels should contain 'notification_channels'")
	}

	// Check for required columns
	requiredColumns := []string{"id", "account_id", "channel_type", "name", "config", "enabled", "created_at", "updated_at"}
	for _, col := range requiredColumns {
		if !strings.Contains(DDLNotificationChannels, col) {
			t.Errorf("DDLNotificationChannels should contain column '%s'", col)
		}
	}

	// Check for CREATE TABLE IF NOT EXISTS
	if !strings.Contains(DDLNotificationChannels, "CREATE TABLE IF NOT EXISTS") {
		t.Error("DDLNotificationChannels should use CREATE TABLE IF NOT EXISTS")
	}

	// Check for index
	if !strings.Contains(DDLNotificationChannels, "idx_notification_channels_account") {
		t.Error("DDLNotificationChannels should have index on account_id")
	}
}

func TestDDLNotificationSubscriptions(t *testing.T) {
	if DDLNotificationSubscriptions == "" {
		t.Error("DDLNotificationSubscriptions should not be empty")
	}

	// Check for required table name
	if !strings.Contains(DDLNotificationSubscriptions, "notification_subscriptions") {
		t.Error("DDLNotificationSubscriptions should contain 'notification_subscriptions'")
	}

	// Check for required columns
	requiredColumns := []string{"id", "account_id", "channel_id", "event_type", "enabled", "created_at", "updated_at"}
	for _, col := range requiredColumns {
		if !strings.Contains(DDLNotificationSubscriptions, col) {
			t.Errorf("DDLNotificationSubscriptions should contain column '%s'", col)
		}
	}

	// Check for CREATE TABLE IF NOT EXISTS
	if !strings.Contains(DDLNotificationSubscriptions, "CREATE TABLE IF NOT EXISTS") {
		t.Error("DDLNotificationSubscriptions should use CREATE TABLE IF NOT EXISTS")
	}

	// Check for unique constraint
	if !strings.Contains(DDLNotificationSubscriptions, "uniq_subscription") {
		t.Error("DDLNotificationSubscriptions should have unique constraint")
	}

	// Check for index
	if !strings.Contains(DDLNotificationSubscriptions, "idx_subscription_account_event") {
		t.Error("DDLNotificationSubscriptions should have index on account_id and event_type")
	}
}

func TestDDLNotificationHistory(t *testing.T) {
	if DDLNotificationHistory == "" {
		t.Error("DDLNotificationHistory should not be empty")
	}

	// Check for required table name
	if !strings.Contains(DDLNotificationHistory, "notification_history") {
		t.Error("DDLNotificationHistory should contain 'notification_history'")
	}

	// Check for required columns
	requiredColumns := []string{"id", "account_id", "channel_id", "event_type", "title", "content", "status", "error", "sent_at", "created_at"}
	for _, col := range requiredColumns {
		if !strings.Contains(DDLNotificationHistory, col) {
			t.Errorf("DDLNotificationHistory should contain column '%s'", col)
		}
	}

	// Check for CREATE TABLE IF NOT EXISTS
	if !strings.Contains(DDLNotificationHistory, "CREATE TABLE IF NOT EXISTS") {
		t.Error("DDLNotificationHistory should use CREATE TABLE IF NOT EXISTS")
	}

	// Check for indexes
	if !strings.Contains(DDLNotificationHistory, "idx_history_account_event") {
		t.Error("DDLNotificationHistory should have index on account_id and event_type")
	}
	if !strings.Contains(DDLNotificationHistory, "idx_history_channel") {
		t.Error("DDLNotificationHistory should have index on channel_id")
	}
}

func TestDDLConsistency(t *testing.T) {
	// All DDL statements should use VARCHAR(64) for IDs
	ddls := []struct {
		name string
		ddl  string
	}{
		{"DDLNotificationChannels", DDLNotificationChannels},
		{"DDLNotificationSubscriptions", DDLNotificationSubscriptions},
		{"DDLNotificationHistory", DDLNotificationHistory},
	}

	for _, d := range ddls {
		t.Run(d.name, func(t *testing.T) {
			// Check for consistent ID type
			if !strings.Contains(d.ddl, "VARCHAR(64)") {
				t.Errorf("%s should use VARCHAR(64) for ID fields", d.name)
			}

			// Check for TIMESTAMP fields
			if !strings.Contains(d.ddl, "TIMESTAMP") {
				t.Errorf("%s should have TIMESTAMP fields", d.name)
			}
		})
	}
}
