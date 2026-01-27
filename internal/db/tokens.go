package db

import (
	"database/sql"
	"time"

	"github.com/rsclarke/oastrix/internal/models"
)

func CreateToken(db *sql.DB, token string, apiKeyID *int64, label *string) (int64, error) {
	result, err := db.Exec(
		"INSERT INTO tokens (token, api_key_id, created_at, label) VALUES (?, ?, ?, ?)",
		token, apiKeyID, time.Now().Unix(), label,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func GetTokenByValue(db *sql.DB, token string) (*models.Token, error) {
	row := db.QueryRow(
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

func DeleteToken(db *sql.DB, token string) error {
	_, err := db.Exec("DELETE FROM tokens WHERE token = ?", token)
	return err
}
