package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"qcc_plus/internal/store"
)

// mockSettingsStore implements store.SettingsStore for testing
type mockSettingsStore struct {
	settings       map[string]*store.Setting
	globalVersion  int64
	shouldFailList bool
	shouldFailGet  bool
}

func newMockSettingsStore() *mockSettingsStore {
	return &mockSettingsStore{
		settings:      make(map[string]*store.Setting),
		globalVersion: 1,
	}
}

func (m *mockSettingsStore) ListSettings(scope, category, accountID string) ([]store.Setting, error) {
	if m.shouldFailList {
		return nil, store.ErrNotFound
	}
	var result []store.Setting
	for _, s := range m.settings {
		if (scope == "" || s.Scope == scope) &&
			(category == "" || s.Category == category) {
			result = append(result, *s)
		}
	}
	return result, nil
}

func (m *mockSettingsStore) GetSetting(key, scope, accountID string) (*store.Setting, error) {
	if m.shouldFailGet {
		return nil, store.ErrNotFound
	}
	settingKey := key + ":" + scope + ":" + accountID
	if s, ok := m.settings[settingKey]; ok {
		return s, nil
	}
	return nil, store.ErrNotFound
}

func (m *mockSettingsStore) UpsertSetting(setting *store.Setting) error {
	accountID := ""
	if setting.AccountID != nil {
		accountID = *setting.AccountID
	}
	settingKey := setting.Key + ":" + setting.Scope + ":" + accountID
	setting.Version = int(m.globalVersion)
	m.settings[settingKey] = setting
	m.globalVersion++
	return nil
}

func (m *mockSettingsStore) UpdateSetting(setting *store.Setting) error {
	accountID := ""
	if setting.AccountID != nil {
		accountID = *setting.AccountID
	}
	settingKey := setting.Key + ":" + setting.Scope + ":" + accountID
	existing, ok := m.settings[settingKey]
	if !ok {
		return store.ErrNotFound
	}
	if existing.Version != setting.Version {
		return store.ErrVersionConflict
	}
	setting.Version++
	m.settings[settingKey] = setting
	m.globalVersion++
	return nil
}

func (m *mockSettingsStore) BatchUpdateSettings(settings []store.Setting) error {
	for i := range settings {
		if err := m.UpdateSetting(&settings[i]); err != nil {
			return err
		}
	}
	return nil
}

func (m *mockSettingsStore) DeleteSetting(key, scope, accountID string) error {
	settingKey := key + ":" + scope + ":" + accountID
	if _, ok := m.settings[settingKey]; !ok {
		return store.ErrNotFound
	}
	delete(m.settings, settingKey)
	m.globalVersion++
	return nil
}

func (m *mockSettingsStore) GetGlobalVersion() (int64, error) {
	return m.globalVersion, nil
}

func TestSettingsHandler_ListSettings(t *testing.T) {
	mockStore := newMockSettingsStore()
	mockStore.settings["test:system:"] = &store.Setting{
		Key:      "test",
		Scope:    "system",
		Value:    "value1",
		Category: "monitor",
		Version:  1,
	}

	handler := &SettingsHandler{
		store: mockStore,
	}

	tests := []struct {
		name       string
		isAdmin    bool
		query      string
		wantStatus int
	}{
		{
			name:       "admin can list",
			isAdmin:    true,
			query:      "",
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-admin forbidden",
			isAdmin:    false,
			query:      "",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "with scope filter",
			isAdmin:    true,
			query:      "?scope=system",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/settings"+tt.query, nil)
			if tt.isAdmin {
				ctx := context.WithValue(req.Context(), isAdminContextKey{}, true)
				req = req.WithContext(ctx)
			}
			w := httptest.NewRecorder()

			handler.ListSettings(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestSettingsHandler_GetRuntimeDefinitions(t *testing.T) {
	handler := &SettingsHandler{}

	tests := []struct {
		name       string
		isAdmin    bool
		wantStatus int
	}{
		{
			name:       "admin can list runtime definitions",
			isAdmin:    true,
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-admin forbidden",
			isAdmin:    false,
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/settings/runtime-definitions", nil)
			if tt.isAdmin {
				req = req.WithContext(context.WithValue(req.Context(), isAdminContextKey{}, true))
			}
			w := httptest.NewRecorder()

			handler.GetRuntimeDefinitions(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if tt.wantStatus != http.StatusOK {
				return
			}

			var payload struct {
				Data []RuntimeSettingDefinition `json:"data"`
			}
			if err := json.NewDecoder(w.Body).Decode(&payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if len(payload.Data) == 0 {
				t.Fatal("expected runtime setting definitions")
			}
		})
	}
}

func TestSettingsHandler_GetSetting(t *testing.T) {
	mockStore := newMockSettingsStore()
	mockStore.settings["test:system:"] = &store.Setting{
		Key:      "test",
		Scope:    "system",
		Value:    "value1",
		Version:  1,
		IsSecret: false,
	}
	mockStore.settings["secret:system:"] = &store.Setting{
		Key:      "secret",
		Scope:    "system",
		Value:    "secret-value",
		Version:  1,
		IsSecret: true,
	}

	handler := &SettingsHandler{
		store: mockStore,
	}

	tests := []struct {
		name       string
		key        string
		scope      string
		isAdmin    bool
		wantStatus int
		wantMasked bool
	}{
		{
			name:       "get existing setting",
			key:        "test",
			scope:      "system",
			isAdmin:    true,
			wantStatus: http.StatusOK,
			wantMasked: false,
		},
		{
			name:       "get secret setting",
			key:        "secret",
			scope:      "system",
			isAdmin:    true,
			wantStatus: http.StatusOK,
			wantMasked: true,
		},
		{
			name:       "non-admin forbidden",
			key:        "test",
			scope:      "system",
			isAdmin:    false,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "not found",
			key:        "nonexistent",
			scope:      "system",
			isAdmin:    true,
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/settings/"+tt.key+"?scope="+tt.scope, nil)
			if tt.isAdmin {
				ctx := context.WithValue(req.Context(), isAdminContextKey{}, true)
				req = req.WithContext(ctx)
			}
			w := httptest.NewRecorder()

			handler.GetSetting(w, req, tt.key)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK && tt.wantMasked {
				var resp map[string]interface{}
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp["data"] == nil {
					t.Fatal("response data is nil")
				}
				data := resp["data"].(map[string]interface{})
				if data["value"] != "******" {
					t.Error("secret value should be masked")
				}
			}
		})
	}
}

func TestSettingsHandler_UpdateSetting(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		body       string
		isAdmin    bool
		setup      func(*mockSettingsStore)
		wantStatus int
	}{
		{
			name:    "create new setting",
			key:     "new",
			body:    `{"value":"test","scope":"system"}`,
			isAdmin: true,
			setup: func(m *mockSettingsStore) {
				// No setup needed
			},
			wantStatus: http.StatusOK,
		},
		{
			name:    "update existing setting",
			key:     "existing",
			body:    `{"value":"updated","scope":"system","version":1}`,
			isAdmin: true,
			setup: func(m *mockSettingsStore) {
				m.settings["existing:system:"] = &store.Setting{
					Key:     "existing",
					Scope:   "system",
					Value:   "old",
					Version: 1,
				}
			},
			wantStatus: http.StatusOK,
		},
		{
			name:    "version conflict",
			key:     "conflict",
			body:    `{"value":"updated","scope":"system","version":1}`,
			isAdmin: true,
			setup: func(m *mockSettingsStore) {
				m.settings["conflict:system:"] = &store.Setting{
					Key:     "conflict",
					Scope:   "system",
					Value:   "old",
					Version: 2, // Different version
				}
			},
			wantStatus: http.StatusConflict,
		},
		{
			name:       "non-admin forbidden",
			key:        "test",
			body:       `{"value":"test"}`,
			isAdmin:    false,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "invalid json",
			key:        "test",
			body:       `invalid json`,
			isAdmin:    true,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := newMockSettingsStore()
			if tt.setup != nil {
				tt.setup(mockStore)
			}

			handler := &SettingsHandler{
				store: mockStore,
			}

			req := httptest.NewRequest("PUT", "/api/settings/"+tt.key, strings.NewReader(tt.body))
			if tt.isAdmin {
				ctx := context.WithValue(req.Context(), isAdminContextKey{}, true)
				req = req.WithContext(ctx)
			}
			w := httptest.NewRecorder()

			handler.UpdateSetting(w, req, tt.key)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d, body: %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestSettingsHandler_DeleteSetting(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		scope      string
		isAdmin    bool
		setup      func(*mockSettingsStore)
		wantStatus int
	}{
		{
			name:    "delete existing",
			key:     "test",
			scope:   "system",
			isAdmin: true,
			setup: func(m *mockSettingsStore) {
				m.settings["test:system:"] = &store.Setting{
					Key:   "test",
					Scope: "system",
					Value: "value",
				}
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "delete non-existent",
			key:        "nonexistent",
			scope:      "system",
			isAdmin:    true,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "non-admin forbidden",
			key:        "test",
			scope:      "system",
			isAdmin:    false,
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := newMockSettingsStore()
			if tt.setup != nil {
				tt.setup(mockStore)
			}

			handler := &SettingsHandler{
				store: mockStore,
			}

			req := httptest.NewRequest("DELETE", "/api/settings/"+tt.key+"?scope="+tt.scope, nil)
			if tt.isAdmin {
				ctx := context.WithValue(req.Context(), isAdminContextKey{}, true)
				req = req.WithContext(ctx)
			}
			w := httptest.NewRecorder()

			handler.DeleteSetting(w, req, tt.key)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestSettingsHandler_BatchUpdate(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		isAdmin    bool
		setup      func(*mockSettingsStore)
		wantStatus int
	}{
		{
			name:    "batch update success",
			body:    `{"settings":[{"key":"test1","scope":"system","value":"v1","version":1}]}`,
			isAdmin: true,
			setup: func(m *mockSettingsStore) {
				m.settings["test1:system:"] = &store.Setting{
					Key:     "test1",
					Scope:   "system",
					Value:   "old",
					Version: 1,
				}
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-admin forbidden",
			body:       `{"settings":[]}`,
			isAdmin:    false,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "invalid json",
			body:       `invalid`,
			isAdmin:    true,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty key",
			body:       `{"settings":[{"key":"","scope":"system"}]}`,
			isAdmin:    true,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := newMockSettingsStore()
			if tt.setup != nil {
				tt.setup(mockStore)
			}

			handler := &SettingsHandler{
				store: mockStore,
			}

			req := httptest.NewRequest("POST", "/api/settings/batch", strings.NewReader(tt.body))
			if tt.isAdmin {
				ctx := context.WithValue(req.Context(), isAdminContextKey{}, true)
				req = req.WithContext(ctx)
			}
			w := httptest.NewRecorder()

			handler.BatchUpdate(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestSettingsHandler_GetVersion(t *testing.T) {
	mockStore := newMockSettingsStore()
	mockStore.globalVersion = 42

	handler := &SettingsHandler{
		store: mockStore,
	}

	tests := []struct {
		name       string
		isAdmin    bool
		wantStatus int
	}{
		{
			name:       "admin can get version",
			isAdmin:    true,
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-admin forbidden",
			isAdmin:    false,
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/settings/version", nil)
			if tt.isAdmin {
				ctx := context.WithValue(req.Context(), isAdminContextKey{}, true)
				req = req.WithContext(ctx)
			}
			w := httptest.NewRecorder()

			handler.GetVersion(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK {
				var resp map[string]interface{}
				json.NewDecoder(w.Body).Decode(&resp)
				version := int64(resp["version"].(float64))
				if version != 42 {
					t.Errorf("version = %d, want 42", version)
				}
			}
		})
	}
}

func TestSettingsHandler_HandleSetting(t *testing.T) {
	mockStore := newMockSettingsStore()
	handler := &SettingsHandler{
		store: mockStore,
	}

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{
			name:       "GET method",
			method:     "GET",
			path:       "/api/settings/test",
			wantStatus: http.StatusNotFound, // Key doesn't exist
		},
		{
			name:       "PUT method",
			method:     "PUT",
			path:       "/api/settings/test",
			wantStatus: http.StatusBadRequest, // Invalid JSON
		},
		{
			name:       "DELETE method",
			method:     "DELETE",
			path:       "/api/settings/test",
			wantStatus: http.StatusNotFound, // Key doesn't exist
		},
		{
			name:       "unsupported method",
			method:     "POST",
			path:       "/api/settings/test",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "empty key",
			method:     "GET",
			path:       "/api/settings/",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			ctx := context.WithValue(req.Context(), isAdminContextKey{}, true)
			req = req.WithContext(ctx)
			w := httptest.NewRecorder()

			handler.HandleSetting(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestSettingsHandler_NilStore(t *testing.T) {
	handler := &SettingsHandler{
		store: nil,
	}

	req := httptest.NewRequest("GET", "/api/settings", nil)
	ctx := context.WithValue(req.Context(), isAdminContextKey{}, true)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ListSettings(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}
