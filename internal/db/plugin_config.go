package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// SetTokenPluginConfig stores plugin configuration for a specific token.
// The config value is JSON-encoded before storage.
func SetTokenPluginConfig(d *sql.DB, tokenID int64, pluginID string, config any) error {
	encoded, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	_, err = d.Exec(`
		INSERT INTO token_plugin_config (token_id, plugin_id, config)
		VALUES (?, ?, ?)
		ON CONFLICT (token_id, plugin_id) DO UPDATE SET config = excluded.config
	`, tokenID, pluginID, string(encoded))
	if err != nil {
		return fmt.Errorf("upsert plugin config: %w", err)
	}

	return nil
}

// GetTokenPluginConfig retrieves plugin configuration for a specific token.
// Returns (true, nil) if found and successfully decoded into out.
// Returns (false, nil) if no configuration exists.
func GetTokenPluginConfig(d *sql.DB, tokenID int64, pluginID string, out any) (bool, error) {
	var config string
	err := d.QueryRow(
		"SELECT config FROM token_plugin_config WHERE token_id = ? AND plugin_id = ?",
		tokenID, pluginID,
	).Scan(&config)

	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("query plugin config: %w", err)
	}

	if err := json.Unmarshal([]byte(config), out); err != nil {
		return false, fmt.Errorf("decode config: %w", err)
	}

	return true, nil
}

// DeleteTokenPluginConfig removes plugin configuration for a specific token.
func DeleteTokenPluginConfig(d *sql.DB, tokenID int64, pluginID string) error {
	_, err := d.Exec(
		"DELETE FROM token_plugin_config WHERE token_id = ? AND plugin_id = ?",
		tokenID, pluginID,
	)
	if err != nil {
		return fmt.Errorf("delete plugin config: %w", err)
	}

	return nil
}
