package db

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestSaveAndGetAttributes(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	_, err = db.Exec("INSERT INTO tokens (token, created_at) VALUES ('test-token', 1234567890)")
	if err != nil {
		t.Fatalf("insert token: %v", err)
	}

	var tokenID int64
	err = db.QueryRow("SELECT id FROM tokens WHERE token='test-token'").Scan(&tokenID)
	if err != nil {
		t.Fatalf("get token id: %v", err)
	}

	interactionID, err := CreateInteraction(db, tokenID, "http", "127.0.0.1", 12345, false, "test")
	if err != nil {
		t.Fatalf("create interaction: %v", err)
	}

	attrs := map[string]any{
		"fingerprint": "abc123",
		"score":       float64(42),
		"nested":      map[string]any{"key": "value"},
		"list":        []any{"a", "b", "c"},
	}

	if err := SaveAttributes(db, interactionID, attrs); err != nil {
		t.Fatalf("SaveAttributes failed: %v", err)
	}

	got, err := GetAttributes(db, interactionID)
	if err != nil {
		t.Fatalf("GetAttributes failed: %v", err)
	}

	if !reflect.DeepEqual(got, attrs) {
		t.Errorf("attributes mismatch\ngot:  %v\nwant: %v", got, attrs)
	}
}

func TestSaveAttributesEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := SaveAttributes(db, 1, map[string]any{}); err != nil {
		t.Fatalf("SaveAttributes with empty map should not error: %v", err)
	}

	if err := SaveAttributes(db, 1, nil); err != nil {
		t.Fatalf("SaveAttributes with nil map should not error: %v", err)
	}
}

func TestSaveAttributesUpsert(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	_, err = db.Exec("INSERT INTO tokens (token, created_at) VALUES ('test-token', 1234567890)")
	if err != nil {
		t.Fatalf("insert token: %v", err)
	}

	var tokenID int64
	err = db.QueryRow("SELECT id FROM tokens WHERE token='test-token'").Scan(&tokenID)
	if err != nil {
		t.Fatalf("get token id: %v", err)
	}

	interactionID, err := CreateInteraction(db, tokenID, "http", "127.0.0.1", 12345, false, "test")
	if err != nil {
		t.Fatalf("create interaction: %v", err)
	}

	if err := SaveAttributes(db, interactionID, map[string]any{"key": "value1"}); err != nil {
		t.Fatalf("first SaveAttributes failed: %v", err)
	}

	if err := SaveAttributes(db, interactionID, map[string]any{"key": "value2"}); err != nil {
		t.Fatalf("second SaveAttributes failed: %v", err)
	}

	got, err := GetAttributes(db, interactionID)
	if err != nil {
		t.Fatalf("GetAttributes failed: %v", err)
	}

	if got["key"] != "value2" {
		t.Errorf("expected key to be updated to 'value2', got %v", got["key"])
	}
}

func TestGetAttributesNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	got, err := GetAttributes(db, 99999)
	if err != nil {
		t.Fatalf("GetAttributes failed: %v", err)
	}

	if len(got) != 0 {
		t.Errorf("expected empty map for non-existent interaction, got %v", got)
	}
}

func TestAttributesCascadeDelete(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	_, err = db.Exec("INSERT INTO tokens (token, created_at) VALUES ('test-token', 1234567890)")
	if err != nil {
		t.Fatalf("insert token: %v", err)
	}

	var tokenID int64
	err = db.QueryRow("SELECT id FROM tokens WHERE token='test-token'").Scan(&tokenID)
	if err != nil {
		t.Fatalf("get token id: %v", err)
	}

	interactionID, err := CreateInteraction(db, tokenID, "http", "127.0.0.1", 12345, false, "test")
	if err != nil {
		t.Fatalf("create interaction: %v", err)
	}

	if err := SaveAttributes(db, interactionID, map[string]any{"key": "value"}); err != nil {
		t.Fatalf("SaveAttributes failed: %v", err)
	}

	_, err = db.Exec("DELETE FROM interactions WHERE id = ?", interactionID)
	if err != nil {
		t.Fatalf("delete interaction: %v", err)
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM interaction_attributes WHERE interaction_id = ?", interactionID).Scan(&count)
	if err != nil {
		t.Fatalf("count attributes: %v", err)
	}

	if count != 0 {
		t.Errorf("expected 0 attributes after cascade delete, got %d", count)
	}
}
