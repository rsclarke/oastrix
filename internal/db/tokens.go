package db

import (
	"database/sql"
	"time"

	"github.com/rsclarke/oastrix/internal/models"
)

// CreateToken inserts a new token into the database and returns its ID.
func CreateToken(d *sql.DB, token string, apiKeyID *int64, label *string) (int64, error) {
	result, err := d.Exec(
		"INSERT INTO tokens (token, api_key_id, created_at, label) VALUES (?, ?, ?, ?)",
		token, apiKeyID, time.Now().Unix(), label,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// GetTokenByValue retrieves a token by its value.
func GetTokenByValue(d *sql.DB, token string) (*models.Token, error) {
	row := d.QueryRow(
		"SELECT id, token, api_key_id, created_at, label FROM tokens WHERE token = ?",
		token,
	)
	var t models.Token
	err := row.Scan(&t.ID, &t.Token, &t.APIKeyID, &t.CreatedAt, &t.Label)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// DeleteToken removes a token from the database by its value.
func DeleteToken(d *sql.DB, token string) error {
	_, err := d.Exec("DELETE FROM tokens WHERE token = ?", token)
	return err
}
