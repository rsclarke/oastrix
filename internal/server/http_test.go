package server

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/rsclarke/oastrix/internal/db"
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

func TestHTTPServer_StoresInteraction(t *testing.T) {
	tmpDB := t.TempDir() + "/test.db"
	database, err := db.Open(tmpDB)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer database.Close()
	defer os.Remove(tmpDB)

	tokenValue := "testtoken123"
	_, err = db.CreateToken(database, tokenValue, nil, nil)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	srv := &HTTPServer{
		DB:     database,
		Domain: "oastrix.example.com",
		Logger: zap.NewNop(),
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
	defer database.Close()
	defer os.Remove(tmpDB)

	srv := &HTTPServer{
		DB:     database,
		Domain: "oastrix.example.com",
		Logger: zap.NewNop(),
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
		database.Close()
	})
	return database
}
