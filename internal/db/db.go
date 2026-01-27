package db

import (
	"database/sql"
	"embed"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func Open(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA foreign_keys=ON;",
		"PRAGMA busy_timeout=5000;",
		"PRAGMA synchronous=NORMAL;",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("exec pragma %q: %w", pragma, err)
		}
	}

	if err := applyMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply migrations: %w", err)
	}

	return db, nil
}

func applyMigrations(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at INTEGER NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var migrations []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			migrations = append(migrations, e.Name())
		}
	}
	sort.Strings(migrations)

	for _, name := range migrations {
		version, err := parseVersion(name)
		if err != nil {
			return fmt.Errorf("parse version from %s: %w", name, err)
		}

		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&count)
		if err != nil {
			return fmt.Errorf("check migration %d: %w", version, err)
		}
		if count > 0 {
			continue
		}

		content, err := migrationsFS.ReadFile(filepath.Join("migrations", name))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		if _, err := db.Exec(string(content)); err != nil {
			return fmt.Errorf("exec migration %s: %w", name, err)
		}

		if _, err := db.Exec("INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)",
			version, time.Now().Unix()); err != nil {
			return fmt.Errorf("record migration %d: %w", version, err)
		}
	}

	return nil
}

func parseVersion(filename string) (int, error) {
	parts := strings.SplitN(filename, "_", 2)
	if len(parts) == 0 {
		return 0, fmt.Errorf("invalid migration filename: %s", filename)
	}
	return strconv.Atoi(parts[0])
}

type TokenWithCount struct {
	Token            string
	Label            *string
	CreatedAt        int64
	InteractionCount int
}

func ListTokensByAPIKey(db *sql.DB, apiKeyID int64) ([]TokenWithCount, error) {
	rows, err := db.Query(`
		SELECT t.token, t.label, t.created_at, COUNT(i.id) as interaction_count
		FROM tokens t
		LEFT JOIN interactions i ON i.token_id = t.id
		WHERE t.api_key_id = ?
		GROUP BY t.id
		ORDER BY t.created_at DESC
	`, apiKeyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []TokenWithCount
	for rows.Next() {
		var t TokenWithCount
		if err := rows.Scan(&t.Token, &t.Label, &t.CreatedAt, &t.InteractionCount); err != nil {
			return nil, err
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}
