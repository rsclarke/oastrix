package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// SaveAttributes stores plugin enrichment data for an interaction.
// Each key-value pair is stored as a separate row with the value JSON-encoded.
func SaveAttributes(d *sql.DB, interactionID int64, attrs map[string]any) error {
	if len(attrs) == 0 {
		return nil
	}

	tx, err := d.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`
		INSERT INTO interaction_attributes (interaction_id, key, value)
		VALUES (?, ?, ?)
		ON CONFLICT (interaction_id, key) DO UPDATE SET value = excluded.value
	`)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for key, val := range attrs {
		encoded, err := json.Marshal(val)
		if err != nil {
			return fmt.Errorf("encode value for key %q: %w", key, err)
		}
		if _, err := stmt.Exec(interactionID, key, string(encoded)); err != nil {
			return fmt.Errorf("insert attribute %q: %w", key, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// GetAttributes retrieves all plugin enrichment data for an interaction.
func GetAttributes(d *sql.DB, interactionID int64) (map[string]any, error) {
	rows, err := d.Query(
		"SELECT key, value FROM interaction_attributes WHERE interaction_id = ?",
		interactionID,
	)
	if err != nil {
		return nil, fmt.Errorf("query attributes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	attrs := make(map[string]any)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("scan attribute: %w", err)
		}
		var decoded any
		if err := json.Unmarshal([]byte(value), &decoded); err != nil {
			return nil, fmt.Errorf("decode value for key %q: %w", key, err)
		}
		attrs[key] = decoded
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate attributes: %w", err)
	}

	return attrs, nil
}
