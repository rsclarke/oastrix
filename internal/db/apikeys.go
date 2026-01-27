package db

import (
	"database/sql"
	"time"

	"github.com/rsclarke/oastrix/internal/models"
)

func CreateAPIKey(db *sql.DB, prefix string, hash []byte) (int64, error) {
	result, err := db.Exec(
		"INSERT INTO api_keys (key_prefix, key_hash, created_at) VALUES (?, ?, ?)",
		prefix, hash, time.Now().Unix(),
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func GetAPIKeyByPrefix(db *sql.DB, prefix string) (*models.APIKey, error) {
	row := db.QueryRow(
		"SELECT id, key_prefix, key_hash, created_at, revoked_at FROM api_keys WHERE key_prefix = ?",
		prefix,
	)
	var key models.APIKey
	err := row.Scan(&key.ID, &key.KeyPrefix, &key.KeyHash, &key.CreatedAt, &key.RevokedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &key, nil
}

func CountAPIKeys(db *sql.DB) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM api_keys WHERE revoked_at IS NULL").Scan(&count)
	return count, err
}
