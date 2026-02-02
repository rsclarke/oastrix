// Package storage implements the storage core plugin that persists interactions to SQLite.
package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/rsclarke/oastrix/internal/db"
	"github.com/rsclarke/oastrix/internal/events"
	"github.com/rsclarke/oastrix/internal/plugins"
)

// Plugin is the storage core plugin that persists interactions to SQLite.
type Plugin struct {
	db     *sql.DB
	logger *zap.Logger
}

// New creates a new storage Plugin with the given database connection.
func New(database *sql.DB) *Plugin {
	return &Plugin{db: database}
}

// ID returns the plugin identifier.
func (p *Plugin) ID() string { return "storage" }

// Init initializes the plugin with the given context.
func (p *Plugin) Init(ctx plugins.InitContext) error {
	p.logger = ctx.Logger.Named("storage")
	return nil
}

// OnPreStore resolves the token value to a token ID if not already set.
func (p *Plugin) OnPreStore(ctx context.Context, e *events.Event) error {
	if e.Draft.TokenID != 0 {
		return nil
	}
	if e.Draft.TokenValue == "" {
		return nil
	}

	token, err := db.GetTokenByValue(p.db, e.Draft.TokenValue)
	if err != nil {
		return fmt.Errorf("resolve token: %w", err)
	}
	if token != nil {
		e.Draft.TokenID = token.ID
	}
	return nil
}

// ResolveTokenID looks up a token by its value and returns the ID.
func (p *Plugin) ResolveTokenID(_ context.Context, tokenValue string) (int64, bool, error) {
	token, err := db.GetTokenByValue(p.db, tokenValue)
	if err != nil {
		return 0, false, err
	}
	if token == nil {
		return 0, false, nil
	}
	return token.ID, true, nil
}

// CreateInteraction persists an interaction draft to the database and returns the interaction ID.
func (p *Plugin) CreateInteraction(_ context.Context, draft *events.InteractionDraft) (int64, error) {
	if draft.TokenID == 0 {
		return 0, nil
	}

	id, err := db.CreateInteraction(
		p.db,
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
				p.db,
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
				p.db,
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
func (p *Plugin) SaveAttributes(_ context.Context, interactionID int64, attrs map[string]any) error {
	return db.SaveAttributes(p.db, interactionID, attrs)
}
