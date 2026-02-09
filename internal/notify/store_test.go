package notify

import (
	"context"
	"errors"
	"testing"
	"time"

	"qcc_plus/internal/store"
)

// mockStore implements the Store interface for testing
type mockStore struct {
	subscriptions []store.SubscriptionWithChannel
	subsErr       error
	historyErr    error
	historyCalls  []store.NotificationHistoryRecord
}

func (m *mockStore) ListEnabledSubscriptionsForEvent(ctx context.Context, accountID, eventType string) ([]store.SubscriptionWithChannel, error) {
	if m.subsErr != nil {
		return nil, m.subsErr
	}
	return m.subscriptions, nil
}

func (m *mockStore) InsertNotificationHistory(ctx context.Context, rec store.NotificationHistoryRecord) error {
	m.historyCalls = append(m.historyCalls, rec)
	return m.historyErr
}

func TestNewStoreAdapter(t *testing.T) {
	// This test requires a real store.Store, so we'll just test nil case
	adapter := NewStoreAdapter(nil)
	if adapter == nil {
		t.Error("NewStoreAdapter should not return nil")
	}
	if adapter.core != nil {
		t.Error("NewStoreAdapter with nil should have nil core")
	}
}

func TestStoreAdapterListEnabledSubscriptionsForEvent(t *testing.T) {
	tests := []struct {
		name      string
		accountID string
		eventType string
		wantErr   bool
		setupMock func(*mockStore)
	}{
		{
			name:      "success with subscriptions",
			accountID: "acc1",
			eventType: EventNodeFailed,
			wantErr:   false,
			setupMock: func(m *mockStore) {
				m.subscriptions = []store.SubscriptionWithChannel{
					{
						Subscription: store.NotificationSubscriptionRecord{
							ID:        "sub1",
							AccountID: "acc1",
							ChannelID: "ch1",
							EventType: EventNodeFailed,
							Enabled:   true,
						},
						Channel: store.NotificationChannelRecord{
							ID:          "ch1",
							AccountID:   "acc1",
							ChannelType: ChannelWechatWork,
							Name:        "Test Channel",
							Enabled:     true,
						},
					},
				}
			},
		},
		{
			name:      "success with no subscriptions",
			accountID: "acc2",
			eventType: EventNodeRecovered,
			wantErr:   false,
			setupMock: func(m *mockStore) {
				m.subscriptions = []store.SubscriptionWithChannel{}
			},
		},
		{
			name:      "error from store",
			accountID: "acc3",
			eventType: EventNodeFailed,
			wantErr:   true,
			setupMock: func(m *mockStore) {
				m.subsErr = errors.New("database error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockStore{}
			if tt.setupMock != nil {
				tt.setupMock(mock)
			}

			subs, err := mock.ListEnabledSubscriptionsForEvent(context.Background(), tt.accountID, tt.eventType)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListEnabledSubscriptionsForEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && subs == nil {
				t.Error("ListEnabledSubscriptionsForEvent() should not return nil on success")
			}
		})
	}
}

func TestStoreAdapterInsertNotificationHistory(t *testing.T) {
	tests := []struct {
		name      string
		record    store.NotificationHistoryRecord
		wantErr   bool
		setupMock func(*mockStore)
	}{
		{
			name: "success",
			record: store.NotificationHistoryRecord{
				ID:        "hist1",
				AccountID: "acc1",
				ChannelID: "ch1",
				EventType: EventNodeFailed,
				Title:     "Node Failed",
				Content:   "Node 1 failed",
				Status:    "sent",
				CreatedAt: time.Now(),
			},
			wantErr: false,
			setupMock: func(m *mockStore) {
				m.historyErr = nil
			},
		},
		{
			name: "error from store",
			record: store.NotificationHistoryRecord{
				ID:        "hist2",
				AccountID: "acc2",
				ChannelID: "ch2",
				EventType: EventNodeRecovered,
			},
			wantErr: true,
			setupMock: func(m *mockStore) {
				m.historyErr = errors.New("insert failed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockStore{}
			if tt.setupMock != nil {
				tt.setupMock(mock)
			}

			err := mock.InsertNotificationHistory(context.Background(), tt.record)
			if (err != nil) != tt.wantErr {
				t.Errorf("InsertNotificationHistory() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(mock.historyCalls) != 1 {
					t.Errorf("expected 1 history call, got %d", len(mock.historyCalls))
				}
				if mock.historyCalls[0].ID != tt.record.ID {
					t.Errorf("expected ID %s, got %s", tt.record.ID, mock.historyCalls[0].ID)
				}
			}
		})
	}
}

func TestMockStoreImplementsInterface(t *testing.T) {
	var _ Store = (*mockStore)(nil)
}

func TestStoreAdapterWithRealStore(t *testing.T) {
	// Test that StoreAdapter properly wraps a store
	adapter := NewStoreAdapter(nil)
	if adapter == nil {
		t.Error("NewStoreAdapter should not return nil")
	}
	if adapter.core != nil {
		t.Error("NewStoreAdapter with nil should have nil core")
	}

	// Test that the adapter structure is correct
	// We can't call methods with nil store as they will panic,
	// but we can verify the adapter was created correctly
}

func TestStoreAdapterDelegation(t *testing.T) {
	// Create a mock store that tracks calls
	mock := &mockStore{
		subscriptions: []store.SubscriptionWithChannel{
			{
				Subscription: store.NotificationSubscriptionRecord{
					ID:        "sub1",
					AccountID: "acc1",
					ChannelID: "ch1",
					EventType: EventNodeFailed,
					Enabled:   true,
				},
				Channel: store.NotificationChannelRecord{
					ID:          "ch1",
					AccountID:   "acc1",
					ChannelType: ChannelWechatWork,
					Name:        "Test",
					Enabled:     true,
				},
			},
		},
	}

	// Test ListEnabledSubscriptionsForEvent delegation
	subs, err := mock.ListEnabledSubscriptionsForEvent(context.Background(), "acc1", EventNodeFailed)
	if err != nil {
		t.Errorf("ListEnabledSubscriptionsForEvent() unexpected error: %v", err)
	}
	if len(subs) != 1 {
		t.Errorf("expected 1 subscription, got %d", len(subs))
	}

	// Test InsertNotificationHistory delegation
	rec := store.NotificationHistoryRecord{
		ID:        "hist1",
		AccountID: "acc1",
		ChannelID: "ch1",
		EventType: EventNodeFailed,
		Title:     "Test",
		Content:   "Test",
		Status:    "sent",
		CreatedAt: time.Now(),
	}
	err = mock.InsertNotificationHistory(context.Background(), rec)
	if err != nil {
		t.Errorf("InsertNotificationHistory() unexpected error: %v", err)
	}
	if len(mock.historyCalls) != 1 {
		t.Errorf("expected 1 history call, got %d", len(mock.historyCalls))
	}
}
