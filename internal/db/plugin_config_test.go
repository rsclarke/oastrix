package db

import (
	"path/filepath"
	"testing"
)

func TestSetAndGetTokenPluginConfig(t *testing.T) {
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

	type config struct {
		Enabled bool   `json:"enabled"`
		Payload string `json:"payload"`
	}

	input := config{Enabled: true, Payload: "<script>alert(1)</script>"}

	if err := SetTokenPluginConfig(db, tokenID, "blindxss", input); err != nil {
		t.Fatalf("SetTokenPluginConfig failed: %v", err)
	}

	var out config
	found, err := GetTokenPluginConfig(db, tokenID, "blindxss", &out)
	if err != nil {
		t.Fatalf("GetTokenPluginConfig failed: %v", err)
	}
	if !found {
		t.Fatal("expected config to be found")
	}

	if out.Enabled != input.Enabled || out.Payload != input.Payload {
		t.Errorf("config mismatch\ngot:  %+v\nwant: %+v", out, input)
	}
}

func TestGetTokenPluginConfigNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	var out map[string]any
	found, err := GetTokenPluginConfig(db, 99999, "nonexistent", &out)
	if err != nil {
		t.Fatalf("GetTokenPluginConfig failed: %v", err)
	}
	if found {
		t.Error("expected config not to be found")
	}
}

func TestSetTokenPluginConfigUpsert(t *testing.T) {
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

	if err := SetTokenPluginConfig(db, tokenID, "testplugin", map[string]any{"value": 1}); err != nil {
		t.Fatalf("first SetTokenPluginConfig failed: %v", err)
	}

	if err := SetTokenPluginConfig(db, tokenID, "testplugin", map[string]any{"value": 2}); err != nil {
		t.Fatalf("second SetTokenPluginConfig failed: %v", err)
	}

	var out map[string]any
	found, err := GetTokenPluginConfig(db, tokenID, "testplugin", &out)
	if err != nil {
		t.Fatalf("GetTokenPluginConfig failed: %v", err)
	}
	if !found {
		t.Fatal("expected config to be found")
	}

	if out["value"] != float64(2) {
		t.Errorf("expected value to be updated to 2, got %v", out["value"])
	}
}

func TestDeleteTokenPluginConfig(t *testing.T) {
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

	if err := SetTokenPluginConfig(db, tokenID, "testplugin", map[string]any{"key": "value"}); err != nil {
		t.Fatalf("SetTokenPluginConfig failed: %v", err)
	}

	if err := DeleteTokenPluginConfig(db, tokenID, "testplugin"); err != nil {
		t.Fatalf("DeleteTokenPluginConfig failed: %v", err)
	}

	var out map[string]any
	found, err := GetTokenPluginConfig(db, tokenID, "testplugin", &out)
	if err != nil {
		t.Fatalf("GetTokenPluginConfig failed: %v", err)
	}
	if found {
		t.Error("expected config to be deleted")
	}
}

func TestDeleteTokenPluginConfigNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := DeleteTokenPluginConfig(db, 99999, "nonexistent"); err != nil {
		t.Fatalf("DeleteTokenPluginConfig should not error for non-existent config: %v", err)
	}
}

func TestTokenPluginConfigCascadeDelete(t *testing.T) {
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

	if err := SetTokenPluginConfig(db, tokenID, "testplugin", map[string]any{"key": "value"}); err != nil {
		t.Fatalf("SetTokenPluginConfig failed: %v", err)
	}

	_, err = db.Exec("DELETE FROM tokens WHERE id = ?", tokenID)
	if err != nil {
		t.Fatalf("delete token: %v", err)
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM token_plugin_config WHERE token_id = ?", tokenID).Scan(&count)
	if err != nil {
		t.Fatalf("count plugin configs: %v", err)
	}

	if count != 0 {
		t.Errorf("expected 0 plugin configs after cascade delete, got %d", count)
	}
}

func TestMultiplePluginConfigs(t *testing.T) {
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

	if err := SetTokenPluginConfig(db, tokenID, "plugin1", map[string]any{"p1": true}); err != nil {
		t.Fatalf("SetTokenPluginConfig plugin1 failed: %v", err)
	}
	if err := SetTokenPluginConfig(db, tokenID, "plugin2", map[string]any{"p2": true}); err != nil {
		t.Fatalf("SetTokenPluginConfig plugin2 failed: %v", err)
	}

	var out1 map[string]any
	found1, err := GetTokenPluginConfig(db, tokenID, "plugin1", &out1)
	if err != nil {
		t.Fatalf("GetTokenPluginConfig plugin1 failed: %v", err)
	}
	if !found1 || out1["p1"] != true {
		t.Errorf("plugin1 config mismatch: found=%v, out=%v", found1, out1)
	}

	var out2 map[string]any
	found2, err := GetTokenPluginConfig(db, tokenID, "plugin2", &out2)
	if err != nil {
		t.Fatalf("GetTokenPluginConfig plugin2 failed: %v", err)
	}
	if !found2 || out2["p2"] != true {
		t.Errorf("plugin2 config mismatch: found=%v, out=%v", found2, out2)
	}
}
