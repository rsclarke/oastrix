package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/rsclarke/oastrix/internal/apitypes"
	"github.com/rsclarke/oastrix/internal/auth"
	"github.com/rsclarke/oastrix/internal/db"
	"github.com/rsclarke/oastrix/internal/plugins"
)

func setupTestAPIServer(t *testing.T) (*APIServer, string, func()) {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "oastrix_api_test_*.db")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	_ = tmpFile.Close()

	database, err := db.Open(tmpFile.Name())
	if err != nil {
		_ = os.Remove(tmpFile.Name())
		t.Fatalf("open database: %v", err)
	}

	displayKey, prefix, hash, err := auth.GenerateAPIKey()
	if err != nil {
		_ = database.Close()
		_ = os.Remove(tmpFile.Name())
		t.Fatalf("generate API key: %v", err)
	}

	_, err = db.CreateAPIKey(database, prefix, hash)
	if err != nil {
		_ = database.Close()
		_ = os.Remove(tmpFile.Name())
		t.Fatalf("create API key: %v", err)
	}

	srv := &APIServer{
		DB:     database,
		Domain: "oastrix.example.com",
	}

	cleanup := func() {
		_ = database.Close()
		_ = os.Remove(tmpFile.Name())
	}

	return srv, displayKey, cleanup
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	srv, _, cleanup := setupTestAPIServer(t)
	defer cleanup()

	req := httptest.NewRequest("POST", "/v1/tokens", nil)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["error"] != "unauthorized" {
		t.Errorf("expected error 'unauthorized', got %q", resp["error"])
	}
}

func TestAuthMiddleware_InvalidKey(t *testing.T) {
	srv, _, cleanup := setupTestAPIServer(t)
	defer cleanup()

	req := httptest.NewRequest("POST", "/v1/tokens", nil)
	req.Header.Set("Authorization", "Bearer invalid_key_format")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_WrongSecret(t *testing.T) {
	srv, displayKey, cleanup := setupTestAPIServer(t)
	defer cleanup()

	prefix, _, _ := auth.ParseAPIKey(displayKey)
	wrongKey := "oastrix_" + prefix + "_wrongsecret"

	req := httptest.NewRequest("POST", "/v1/tokens", nil)
	req.Header.Set("Authorization", "Bearer "+wrongKey)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_ValidKey(t *testing.T) {
	srv, displayKey, cleanup := setupTestAPIServer(t)
	defer cleanup()

	req := httptest.NewRequest("POST", "/v1/tokens", nil)
	req.Header.Set("Authorization", "Bearer "+displayKey)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestCreateToken(t *testing.T) {
	srv, displayKey, cleanup := setupTestAPIServer(t)
	defer cleanup()

	body := bytes.NewBufferString(`{"label": "test token"}`)
	req := httptest.NewRequest("POST", "/v1/tokens", body)
	req.Header.Set("Authorization", "Bearer "+displayKey)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp apitypes.CreateTokenResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Token == "" {
		t.Error("expected token to be non-empty")
	}

	if resp.Payloads["http"] == "" {
		t.Error("expected http payload")
	}
	if resp.Payloads["https"] == "" {
		t.Error("expected https payload")
	}
	if resp.Payloads["dns"] == "" {
		t.Error("expected dns payload")
	}
}

func TestGetInteractions(t *testing.T) {
	srv, displayKey, cleanup := setupTestAPIServer(t)
	defer cleanup()

	createReq := httptest.NewRequest("POST", "/v1/tokens", nil)
	createReq.Header.Set("Authorization", "Bearer "+displayKey)
	createW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createW, createReq)

	var createResp apitypes.CreateTokenResponse
	if err := json.NewDecoder(createW.Body).Decode(&createResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	req := httptest.NewRequest("GET", "/v1/tokens/"+createResp.Token+"/interactions", nil)
	req.Header.Set("Authorization", "Bearer "+displayKey)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp apitypes.GetInteractionsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Token != createResp.Token {
		t.Errorf("expected token %q, got %q", createResp.Token, resp.Token)
	}

	if resp.Interactions == nil {
		t.Error("expected interactions to be non-nil")
	}
}

func TestGetInteractions_NotFound(t *testing.T) {
	srv, displayKey, cleanup := setupTestAPIServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/v1/tokens/nonexistent123/interactions", nil)
	req.Header.Set("Authorization", "Bearer "+displayKey)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestDeleteToken(t *testing.T) {
	srv, displayKey, cleanup := setupTestAPIServer(t)
	defer cleanup()

	createReq := httptest.NewRequest("POST", "/v1/tokens", nil)
	createReq.Header.Set("Authorization", "Bearer "+displayKey)
	createW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createW, createReq)

	var createResp apitypes.CreateTokenResponse
	if err := json.NewDecoder(createW.Body).Decode(&createResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	req := httptest.NewRequest("DELETE", "/v1/tokens/"+createResp.Token, nil)
	req.Header.Set("Authorization", "Bearer "+displayKey)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]bool
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp["deleted"] {
		t.Error("expected deleted to be true")
	}

	getReq := httptest.NewRequest("GET", "/v1/tokens/"+createResp.Token+"/interactions", nil)
	getReq.Header.Set("Authorization", "Bearer "+displayKey)
	getW := httptest.NewRecorder()
	srv.Handler().ServeHTTP(getW, getReq)

	if getW.Code != http.StatusNotFound {
		t.Errorf("expected status 404 after delete, got %d", getW.Code)
	}
}

func TestDeleteToken_NotFound(t *testing.T) {
	srv, displayKey, cleanup := setupTestAPIServer(t)
	defer cleanup()

	req := httptest.NewRequest("DELETE", "/v1/tokens/nonexistent123", nil)
	req.Header.Set("Authorization", "Bearer "+displayKey)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestTokenOwnership_CannotAccessOtherKeysToken(t *testing.T) {
	srv, displayKey1, cleanup := setupTestAPIServer(t)
	defer cleanup()

	// Create a token with API key 1
	body := bytes.NewBufferString(`{"label":"owned by key1"}`)
	createReq := httptest.NewRequest("POST", "/v1/tokens", body)
	createReq.Header.Set("Authorization", "Bearer "+displayKey1)
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()

	srv.Handler().ServeHTTP(createW, createReq)

	if createW.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", createW.Code)
	}

	var createResp apitypes.CreateTokenResponse
	if err := json.NewDecoder(createW.Body).Decode(&createResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	tokenValue := createResp.Token

	// Create a second API key
	displayKey2, prefix2, hash2, err := auth.GenerateAPIKey()
	if err != nil {
		t.Fatalf("generate second API key: %v", err)
	}
	_, err = db.CreateAPIKey(srv.DB, prefix2, hash2)
	if err != nil {
		t.Fatalf("create second API key: %v", err)
	}

	// Try to access the token with API key 2 - should return 404 (not found, not forbidden)
	getReq := httptest.NewRequest("GET", "/v1/tokens/"+tokenValue+"/interactions", nil)
	getReq.Header.Set("Authorization", "Bearer "+displayKey2)
	getW := httptest.NewRecorder()

	srv.Handler().ServeHTTP(getW, getReq)

	if getW.Code != http.StatusNotFound {
		t.Errorf("expected status 404 when accessing another key's token, got %d", getW.Code)
	}

	// Try to delete the token with API key 2 - should also return 404
	deleteReq := httptest.NewRequest("DELETE", "/v1/tokens/"+tokenValue, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+displayKey2)
	deleteW := httptest.NewRecorder()

	srv.Handler().ServeHTTP(deleteW, deleteReq)

	if deleteW.Code != http.StatusNotFound {
		t.Errorf("expected status 404 when deleting another key's token, got %d", deleteW.Code)
	}

	// Original key should still be able to access
	getReq2 := httptest.NewRequest("GET", "/v1/tokens/"+tokenValue+"/interactions", nil)
	getReq2.Header.Set("Authorization", "Bearer "+displayKey1)
	getW2 := httptest.NewRecorder()

	srv.Handler().ServeHTTP(getW2, getReq2)

	if getW2.Code != http.StatusOK {
		t.Errorf("expected status 200 when accessing own token, got %d", getW2.Code)
	}
}

type mockPlugin struct {
	id     string
	isCore bool
	config map[string]any
}

func (m *mockPlugin) ID() string                       { return m.id }
func (m *mockPlugin) Init(_ plugins.InitContext) error { return nil }
func (m *mockPlugin) IsCore() bool                     { return m.isCore }
func (m *mockPlugin) Config() map[string]any           { return m.config }

func TestListPlugins(t *testing.T) {
	srv, displayKey, cleanup := setupTestAPIServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/v1/plugins", nil)
	req.Header.Set("Authorization", "Bearer "+displayKey)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp apitypes.ListPluginsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Plugins == nil {
		t.Error("expected plugins to be non-nil")
	}

	if len(resp.Plugins) != 0 {
		t.Errorf("expected empty plugins list for server without registry, got %d", len(resp.Plugins))
	}
}

func TestListPlugins_WithRegistry(t *testing.T) {
	srv, displayKey, cleanup := setupTestAPIServer(t)
	defer cleanup()

	pipeline := plugins.NewPipeline(nil)
	corePlugin := &mockPlugin{id: "storage", isCore: true}
	featurePlugin := &mockPlugin{
		id:     "dnsexfil",
		isCore: false,
		config: map[string]any{"encodings": []string{"base64", "base32", "hex"}},
	}
	pipeline.Register(corePlugin)
	pipeline.Register(featurePlugin)
	srv.Plugins = pipeline

	req := httptest.NewRequest("GET", "/v1/plugins", nil)
	req.Header.Set("Authorization", "Bearer "+displayKey)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp apitypes.ListPluginsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(resp.Plugins))
	}

	if resp.Plugins[0].ID != "storage" {
		t.Errorf("expected first plugin ID 'storage', got %q", resp.Plugins[0].ID)
	}
	if resp.Plugins[0].Type != "core" {
		t.Errorf("expected first plugin type 'core', got %q", resp.Plugins[0].Type)
	}
	if !resp.Plugins[0].Enabled {
		t.Error("expected first plugin to be enabled")
	}

	if resp.Plugins[1].ID != "dnsexfil" {
		t.Errorf("expected second plugin ID 'dnsexfil', got %q", resp.Plugins[1].ID)
	}
	if resp.Plugins[1].Type != "feature" {
		t.Errorf("expected second plugin type 'feature', got %q", resp.Plugins[1].Type)
	}
	if resp.Plugins[1].Config == nil {
		t.Error("expected second plugin to have config")
	}
}
