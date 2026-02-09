package store

import (
	"context"
	"testing"
)

func TestSettingsFunctions(t *testing.T) {
	s := setupTestStore(t)
	defer s.Close()

	ctx := context.Background()

	// Create test account
	acc := AccountRecord{
		ID:   "test-acc-settings",
		Name: "Settings Test Account",
	}
	if err := s.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("seed default settings", func(t *testing.T) {
		err := s.SeedDefaultSettings()
		if err != nil {
			t.Fatalf("SeedDefaultSettings failed: %v", err)
		}
	})

	t.Run("list settings", func(t *testing.T) {
		settings, err := s.ListSettings("", "", "")
		if err != nil {
			t.Fatalf("ListSettings failed: %v", err)
		}

		if len(settings) == 0 {
			t.Error("Expected at least one setting")
		}
	})

	t.Run("list settings by account", func(t *testing.T) {
		// First upsert a setting for the account
		accountID := "test-acc-settings"
		setting := &Setting{
			Key:       "test_key",
			Scope:     "account",
			AccountID: &accountID,
			Value:     "test_value",
			DataType:  "string",
			Category:  "general",
		}
		if err := s.UpsertSetting(setting); err != nil {
			t.Fatalf("UpsertSetting failed: %v", err)
		}

		settings, err := s.ListSettings("", "", "test-acc-settings")
		if err != nil {
			t.Fatalf("ListSettings failed: %v", err)
		}

		found := false
		for _, s := range settings {
			if s.Key == "test_key" && s.AccountID != nil && *s.AccountID == "test-acc-settings" {
				found = true
				break
			}
		}

		if !found {
			t.Error("Expected to find test_key setting for account")
		}
	})

	t.Run("get setting", func(t *testing.T) {
		setting, err := s.GetSetting("test_key", "account", "test-acc-settings")
		if err != nil {
			t.Fatalf("GetSetting failed: %v", err)
		}

		if setting.Key != "test_key" {
			t.Errorf("Expected key test_key, got %s", setting.Key)
		}
		if setting.Value != "test_value" {
			t.Errorf("Expected value test_value, got %v", setting.Value)
		}
	})

	t.Run("upsert setting", func(t *testing.T) {
		// Test creating a new setting
		setting := &Setting{
			Key:      "upsert_test_key_unique",
			Scope:    "system",
			Value:    "initial_value",
			DataType: "string",
			Category: "general",
		}

		err := s.UpsertSetting(setting)
		if err != nil {
			t.Fatalf("UpsertSetting failed: %v", err)
		}

		// Verify creation
		retrieved, err := s.GetSetting("upsert_test_key_unique", "system", "")
		if err != nil {
			t.Fatalf("GetSetting failed: %v", err)
		}

		if retrieved.Value != "initial_value" {
			t.Errorf("Expected value initial_value, got %v", retrieved.Value)
		}

		// Test updating an existing setting
		// Get the setting again to ensure we have the latest version
		toUpdate, err := s.GetSetting("upsert_test_key_unique", "system", "")
		if err != nil {
			t.Fatalf("GetSetting failed: %v", err)
		}

		toUpdate.Value = "updated_value"
		err = s.UpsertSetting(toUpdate)
		if err != nil {
			t.Fatalf("UpsertSetting (update) failed: %v", err)
		}

		// Verify update
		retrieved, err = s.GetSetting("upsert_test_key_unique", "system", "")
		if err != nil {
			t.Fatalf("GetSetting failed: %v", err)
		}

		if retrieved.Value != "updated_value" {
			t.Errorf("Expected value updated_value, got %v", retrieved.Value)
		}
	})

	t.Run("update setting", func(t *testing.T) {
		// Create a setting first
		setting := &Setting{
			Key:      "update_key",
			Scope:    "system",
			Value:    "original_value",
			DataType: "string",
			Category: "general",
		}
		if err := s.UpsertSetting(setting); err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Get it to get the version
		retrieved, err := s.GetSetting("update_key", "system", "")
		if err != nil {
			t.Fatalf("GetSetting failed: %v", err)
		}

		// Update it
		retrieved.Value = "new_value"
		err = s.UpdateSetting(retrieved)
		if err != nil {
			t.Fatalf("UpdateSetting failed: %v", err)
		}

		// Verify update
		retrieved, err = s.GetSetting("update_key", "system", "")
		if err != nil {
			t.Fatalf("GetSetting failed: %v", err)
		}

		if retrieved.Value != "new_value" {
			t.Errorf("Expected value new_value, got %v", retrieved.Value)
		}
	})

	t.Run("delete setting", func(t *testing.T) {
		// Create a setting first
		setting := &Setting{
			Key:      "delete_key",
			Scope:    "system",
			Value:    "delete_value",
			DataType: "string",
			Category: "general",
		}
		if err := s.UpsertSetting(setting); err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Delete it
		err := s.DeleteSetting("delete_key", "system", "")
		if err != nil {
			t.Fatalf("DeleteSetting failed: %v", err)
		}

		// Verify deletion
		_, err = s.GetSetting("delete_key", "system", "")
		if err != ErrNotFound {
			t.Errorf("Expected ErrNotFound after deletion, got %v", err)
		}
	})

	t.Run("batch update settings", func(t *testing.T) {
		settings := []Setting{
			{Key: "batch_key_1", Scope: "system", Value: "batch_value_1", DataType: "string", Category: "general"},
			{Key: "batch_key_2", Scope: "system", Value: "batch_value_2", DataType: "string", Category: "general"},
			{Key: "batch_key_3", Scope: "system", Value: "batch_value_3", DataType: "string", Category: "general"},
		}

		err := s.BatchUpdateSettings(settings)
		if err != nil {
			t.Fatalf("BatchUpdateSettings failed: %v", err)
		}

		// Verify all settings were created/updated
		for _, setting := range settings {
			retrieved, err := s.GetSetting(setting.Key, "system", "")
			if err != nil {
				t.Errorf("GetSetting failed for %s: %v", setting.Key, err)
				continue
			}

			if retrieved.Value != setting.Value {
				t.Errorf("Expected value %v for key %s, got %v", setting.Value, setting.Key, retrieved.Value)
			}
		}
	})

	t.Run("get global version", func(t *testing.T) {
		version, err := s.GetGlobalVersion()
		if err != nil {
			t.Fatalf("GetGlobalVersion failed: %v", err)
		}

		if version < 0 {
			t.Errorf("Expected non-negative version, got %d", version)
		}
	})
}

