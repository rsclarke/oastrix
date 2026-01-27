package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCreatesDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}

func TestMigrationsApplied(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	tables := []string{"schema_migrations", "api_keys", "tokens", "interactions", "http_interactions", "dns_interactions"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %s not found: %v", table, err)
		}
	}
}

func TestForeignKeysEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	var fkEnabled int
	err = db.QueryRow("PRAGMA foreign_keys").Scan(&fkEnabled)
	if err != nil {
		t.Fatalf("PRAGMA foreign_keys failed: %v", err)
	}
	if fkEnabled != 1 {
		t.Error("foreign keys not enabled")
	}
}

func TestCascadeDelete(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	_, err = db.Exec("INSERT INTO tokens (token, created_at) VALUES ('test-token', 1234567890)")
	if err != nil {
		t.Fatalf("insert token: %v", err)
	}

	var tokenID int64
	err = db.QueryRow("SELECT id FROM tokens WHERE token='test-token'").Scan(&tokenID)
	if err != nil {
		t.Fatalf("get token id: %v", err)
	}

	_, err = db.Exec("INSERT INTO interactions (token_id, kind, occurred_at, remote_ip) VALUES (?, 'http', 1234567890, '127.0.0.1')", tokenID)
	if err != nil {
		t.Fatalf("insert interaction: %v", err)
	}

	_, err = db.Exec("DELETE FROM tokens WHERE id=?", tokenID)
	if err != nil {
		t.Fatalf("delete token: %v", err)
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM interactions WHERE token_id=?", tokenID).Scan(&count)
	if err != nil {
		t.Fatalf("count interactions: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 interactions after cascade delete, got %d", count)
	}
}
