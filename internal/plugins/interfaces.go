// Package plugins defines the plugin interfaces and capability hooks for the plugin framework.
package plugins

import (
	"context"
	"net/http"

	"go.uber.org/zap"

	"github.com/rsclarke/oastrix/internal/events"
)

// Plugin is the base interface all plugins must implement.
type Plugin interface {
	ID() string
	Init(ctx InitContext) error
}

// InitContext provides access to shared resources during plugin initialization.
type InitContext struct {
	Logger *zap.Logger
	Store  Store
	Config GlobalConfigView
	Tokens TokenConfigView
	Router RouterRegistrar
}

// Store provides storage operations for plugins.
type Store interface {
	ResolveTokenID(ctx context.Context, tokenValue string) (int64, bool, error)
	CreateInteraction(ctx context.Context, draft *events.InteractionDraft) (int64, error)
	SaveAttributes(ctx context.Context, interactionID int64, attrs map[string]any) error
}

// RouterRegistrar allows plugins to register HTTP handlers.
type RouterRegistrar interface {
	Handle(pattern string, h http.Handler)
}

// GlobalConfigView provides read access to global configuration.
type GlobalConfigView interface {
	Get(key string, out any) error
}

// TokenConfigView provides read access to per-token plugin configuration.
type TokenConfigView interface {
	Get(ctx context.Context, tokenID int64, pluginID string, out any) (bool, error)
}

// PreStoreHook is called after token extraction, before persistence.
type PreStoreHook interface {
	OnPreStore(ctx context.Context, e *events.Event) error
}

// PostStoreHook is called after the interaction is persisted.
type PostStoreHook interface {
	OnPostStore(ctx context.Context, e *events.Event) error
}

// HTTPResponseHook is called before writing the HTTP response.
type HTTPResponseHook interface {
	OnHTTPResponse(ctx context.Context, e *events.HTTPEvent) error
}

// DNSResponseHook is called before writing the DNS response.
type DNSResponseHook interface {
	OnDNSResponse(ctx context.Context, e *events.DNSEvent) error
}

// PluginType indicates whether a plugin is core infrastructure or a feature plugin.
type PluginType string

// Plugin type constants.
const (
	PluginTypeCore    PluginType = "core"
	PluginTypeFeature PluginType = "feature"
)

// CorePlugin is an optional interface that core plugins can implement.
type CorePlugin interface {
	IsCore() bool
}

// ConfigurablePlugin is an optional interface for plugins that expose global configuration.
type ConfigurablePlugin interface {
	Config() map[string]any
}

// PluginInfo contains metadata about a registered plugin.
type PluginInfo struct {
	ID      string         `json:"id"`
	Type    PluginType     `json:"type"`
	Enabled bool           `json:"enabled"`
	Config  map[string]any `json:"config,omitempty"`
}

// PluginRegistry provides read access to registered plugins.
type PluginRegistry interface {
	ListPlugins() []PluginInfo
}
