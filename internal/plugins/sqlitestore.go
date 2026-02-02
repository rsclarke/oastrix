package plugins

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/rsclarke/oastrix/internal/db"
	"github.com/rsclarke/oastrix/internal/events"
)

// SQLiteStore implements the Store interface using SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLiteStore with the given database connection.
func NewSQLiteStore(database *sql.DB) *SQLiteStore {
	return &SQLiteStore{db: database}
}

// ResolveTokenID looks up a token by its value and returns the ID.
func (s *SQLiteStore) ResolveTokenID(_ context.Context, tokenValue string) (int64, bool, error) {
	token, err := db.GetTokenByValue(s.db, tokenValue)
	if err != nil {
		return 0, false, err
	}
	if token == nil {
		return 0, false, nil
	}
	return token.ID, true, nil
}

// CreateInteraction persists an interaction draft and returns the interaction ID.
func (s *SQLiteStore) CreateInteraction(_ context.Context, draft *events.InteractionDraft) (int64, error) {
	if draft.TokenID == 0 && draft.TokenValue != "" {
		token, err := db.GetTokenByValue(s.db, draft.TokenValue)
		if err != nil {
			return 0, fmt.Errorf("resolve token: %w", err)
		}
		if token == nil {
			return 0, nil
		}
		draft.TokenID = token.ID
	}

	if draft.TokenID == 0 {
		return 0, nil
	}

	id, err := db.CreateInteraction(
		s.db,
		draft.TokenID,
		string(draft.Kind),
		draft.RemoteIP,
		draft.RemotePort,
		draft.TLS,
		draft.Summary,
	)
	if err != nil {
		return 0, fmt.Errorf("create interaction: %w", err)
	}

	switch draft.Kind {
	case events.KindHTTP:
		if draft.HTTP != nil {
			headers, err := json.Marshal(draft.HTTP.Headers)
			if err != nil {
				return 0, fmt.Errorf("marshal headers: %w", err)
			}
			err = db.CreateHTTPInteraction(
				s.db,
				id,
				draft.HTTP.Method,
				draft.HTTP.Scheme,
				draft.HTTP.Host,
				draft.HTTP.Path,
				draft.HTTP.Query,
				draft.HTTP.Proto,
				string(headers),
				draft.HTTP.Body,
			)
			if err != nil {
				return 0, fmt.Errorf("create http interaction: %w", err)
			}
		}
	case events.KindDNS:
		if draft.DNS != nil {
			err = db.CreateDNSInteraction(
				s.db,
				id,
				draft.DNS.QName,
				draft.DNS.QType,
				draft.DNS.QClass,
				draft.DNS.RD,
				draft.DNS.Opcode,
				draft.DNS.DNSID,
				draft.DNS.Protocol,
			)
			if err != nil {
				return 0, fmt.Errorf("create dns interaction: %w", err)
			}
		}
	}

	return id, nil
}

// SaveAttributes persists plugin attributes for an interaction.
func (s *SQLiteStore) SaveAttributes(_ context.Context, interactionID int64, attrs map[string]any) error {
	return db.SaveAttributes(s.db, interactionID, attrs)
}
