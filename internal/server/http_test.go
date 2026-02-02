package server

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/rsclarke/oastrix/internal/db"
	"github.com/rsclarke/oastrix/internal/plugins"
	"github.com/rsclarke/oastrix/internal/plugins/core/defaultresponse"
	"github.com/rsclarke/oastrix/internal/plugins/core/storage"
	"go.uber.org/zap"
)

func TestExtractToken_FromHost(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		domain   string
		expected string
	}{
		{
			name:     "simple subdomain",
			host:     "abc123.oastrix.example.com",
			domain:   "oastrix.example.com",
			expected: "abc123",
		},
		{
			name:     "with port",
			host:     "abc123.oastrix.example.com:8080",
			domain:   "oastrix.example.com",
			expected: "abc123",
		},
		{
			name:     "nested subdomain takes last part",
			host:     "www.abc123.oastrix.example.com",
			domain:   "oastrix.example.com",
			expected: "abc123",
		},
		{
			name:     "no match - different domain",
			host:     "abc123.other.com",
			domain:   "oastrix.example.com",
			expected: "",
		},
		{
			name:     "exact domain match - no subdomain",
			host:     "oastrix.example.com",
			domain:   "oastrix.example.com",
			expected: "",
		},
		{
			name:     "IPv4 with port",
			host:     "1.2.3.4:443",
			domain:   "oastrix.example.com",
			expected: "",
		},
		{
			name:     "IPv6 with port",
			host:     "[2001:db8::1]:443",
			domain:   "oastrix.example.com",
			expected: "",
		},
		{
			name:     "IPv6 without port",
			host:     "2001:db8::1",
			domain:   "oastrix.example.com",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "http://"+tt.host+"/path", nil)
			r.Host = tt.host
			got := ExtractToken(r, tt.domain)
			if got != tt.expected {
				t.Errorf("ExtractToken() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestExtractToken_FromPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "oast path prefix",
			path:     "/oast/abc123",
			expected: "abc123",
		},
		{
			name:     "oast path with trailing path",
			path:     "/oast/abc123/extra/path",
			expected: "abc123",
		},
		{
			name:     "no oast prefix",
			path:     "/other/abc123",
			expected: "",
		},
		{
			name:     "empty oast token",
			path:     "/oast/",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "http://example.com"+tt.path, nil)
			r.Host = "example.com"
			got := ExtractToken(r, "oastrix.example.com")
			if got != tt.expected {
				t.Errorf("ExtractToken() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func setupPipeline(t *testing.T, database *sql.DB) *plugins.Pipeline {
	t.Helper()
	logger := zap.NewNop()
	pipeline := plugins.NewPipeline(logger)

	storagePlugin := storage.New(database)
	_ = storagePlugin.Init(plugins.InitContext{Logger: logger})
	pipeline.SetStore(storagePlugin)
	pipeline.Register(storagePlugin)

	defaultResp := defaultresponse.New("127.0.0.1")
	_ = defaultResp.Init(plugins.InitContext{Logger: logger})
	pipeline.Register(defaultResp)

	return pipeline
}

func TestHTTPServer_StoresInteraction(t *testing.T) {
	tmpDB := t.TempDir() + "/test.db"
	database, err := db.Open(tmpDB)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer func() { _ = database.Close() }()
	defer func() { _ = os.Remove(tmpDB) }()

	tokenValue := "testtoken123"
	_, err = db.CreateToken(database, tokenValue, nil, nil)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	srv := &HTTPServer{
		Pipeline: setupPipeline(t, database),
		Domain:   "oastrix.example.com",
		Logger:   zap.NewNop(),
	}

	req := httptest.NewRequest("POST", "http://testtoken123.oastrix.example.com/test/path?foo=bar", strings.NewReader("request body"))
	req.Host = "testtoken123.oastrix.example.com"
	req.Header.Set("X-Custom-Header", "custom-value")

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("expected body 'ok', got %q", rec.Body.String())
	}

	var count int
	err = database.QueryRow("SELECT COUNT(*) FROM interactions").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count interactions: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 interaction, got %d", count)
	}

	var method, path, query string
	var body []byte
	err = database.QueryRow("SELECT method, path, query, request_body FROM http_interactions").Scan(&method, &path, &query, &body)
	if err != nil {
		t.Fatalf("failed to query http_interactions: %v", err)
	}
	if method != "POST" {
		t.Errorf("expected method POST, got %s", method)
	}
	if path != "/test/path" {
		t.Errorf("expected path /test/path, got %s", path)
	}
	if query != "foo=bar" {
		t.Errorf("expected query foo=bar, got %s", query)
	}
	if string(body) != "request body" {
		t.Errorf("expected body 'request body', got %s", string(body))
	}
}

func TestHTTPServer_UnknownTokenDoesNotError(t *testing.T) {
	tmpDB := t.TempDir() + "/test.db"
	database, err := db.Open(tmpDB)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer func() { _ = database.Close() }()
	defer func() { _ = os.Remove(tmpDB) }()

	srv := &HTTPServer{
		Pipeline: setupPipeline(t, database),
		Domain:   "oastrix.example.com",
		Logger:   zap.NewNop(),
	}

	req := httptest.NewRequest("GET", "http://unknowntoken.oastrix.example.com/", nil)
	req.Host = "unknowntoken.oastrix.example.com"

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var count int
	err = database.QueryRow("SELECT COUNT(*) FROM interactions").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count interactions: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 interactions for unknown token, got %d", count)
	}
}

func setupTestDB(t *testing.T) *sql.DB {
	tmpDB := t.TempDir() + "/test.db"
	database, err := db.Open(tmpDB)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})
	return database
}

func TestIsValidHost(t *testing.T) {
	srv := &HTTPServer{
		Domain:   "oastrix.example.com",
		PublicIP: "203.0.113.10",
	}

	tests := []struct {
		name  string
		host  string
		valid bool
	}{
		{"subdomain", "token.oastrix.example.com", true},
		{"subdomain with port", "token.oastrix.example.com:443", true},
		{"exact domain", "oastrix.example.com", true},
		{"exact domain with port", "oastrix.example.com:443", true},
		{"public IP", "203.0.113.10", true},
		{"public IP with port", "203.0.113.10:443", true},
		{"unrecognized domain", "evil.com", false},
		{"unrecognized IP", "1.2.3.4", false},
		{"empty host", "", false},
		{"IPv6 public IP", "[2001:db8::1]", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := srv.isValidHost(tt.host)
			if got != tt.valid {
				t.Errorf("isValidHost(%q) = %v, want %v", tt.host, got, tt.valid)
			}
		})
	}
}

func TestIsValidHost_IPv6PublicIP(t *testing.T) {
	srv := &HTTPServer{
		Domain:   "oastrix.example.com",
		PublicIP: "2001:db8::1",
	}

	tests := []struct {
		name  string
		host  string
		valid bool
	}{
		{"IPv6 with brackets and port", "[2001:db8::1]:443", true},
		{"IPv6 with brackets", "[2001:db8::1]", true},
		{"IPv6 bare", "2001:db8::1", true},
		{"wrong IPv6", "2001:db8::2", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := srv.isValidHost(tt.host)
			if got != tt.valid {
				t.Errorf("isValidHost(%q) = %v, want %v", tt.host, got, tt.valid)
			}
		})
	}
}

func TestHTTPServer_InvalidHostReturns404(t *testing.T) {
	database := setupTestDB(t)

	srv := &HTTPServer{
		Pipeline: setupPipeline(t, database),
		Domain:   "oastrix.example.com",
		PublicIP: "203.0.113.10",
		Logger:   zap.NewNop(),
	}

	req := httptest.NewRequest("GET", "http://evil.com/oast/token123", nil)
	req.Host = "evil.com"

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for invalid host, got %d", rec.Code)
	}
}

func TestHTTPServer_ValidHostsAccepted(t *testing.T) {
	database := setupTestDB(t)

	srv := &HTTPServer{
		Pipeline: setupPipeline(t, database),
		Domain:   "oastrix.example.com",
		PublicIP: "203.0.113.10",
		Logger:   zap.NewNop(),
	}

	tests := []struct {
		name string
		host string
	}{
		{"subdomain", "token.oastrix.example.com"},
		{"domain with path", "oastrix.example.com"},
		{"public IP", "203.0.113.10"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://"+tt.host+"/", nil)
			req.Host = tt.host

			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected status 200 for valid host %q, got %d", tt.host, rec.Code)
			}
		})
	}
}
