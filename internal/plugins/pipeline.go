package plugins

import (
	"context"

	"go.uber.org/zap"

	"github.com/rsclarke/oastrix/internal/events"
)

// Pipeline orchestrates plugin hook execution in the correct order.
type Pipeline struct {
	store        Store
	plugins      []Plugin
	preStore     []PreStoreHook
	postStore    []PostStoreHook
	httpResponse []HTTPResponseHook
	dnsResponse  []DNSResponseHook
	logger       *zap.Logger
}

// NewPipeline creates a new Pipeline with the given logger.
func NewPipeline(logger *zap.Logger) *Pipeline {
	return &Pipeline{
		logger:       logger,
		plugins:      make([]Plugin, 0),
		preStore:     make([]PreStoreHook, 0),
		postStore:    make([]PostStoreHook, 0),
		httpResponse: make([]HTTPResponseHook, 0),
		dnsResponse:  make([]DNSResponseHook, 0),
	}
}

// SetStore sets the storage backend for the pipeline.
func (p *Pipeline) SetStore(store Store) {
	p.store = store
}

// Register detects which capability interfaces a plugin implements
// and adds it to the appropriate hook lists.
func (p *Pipeline) Register(plugin Plugin) {
	p.plugins = append(p.plugins, plugin)
	if hook, ok := plugin.(PreStoreHook); ok {
		p.preStore = append(p.preStore, hook)
	}
	if hook, ok := plugin.(PostStoreHook); ok {
		p.postStore = append(p.postStore, hook)
	}
	if hook, ok := plugin.(HTTPResponseHook); ok {
		p.httpResponse = append(p.httpResponse, hook)
	}
	if hook, ok := plugin.(DNSResponseHook); ok {
		p.dnsResponse = append(p.dnsResponse, hook)
	}
}

// ListPlugins returns metadata about all registered plugins.
func (p *Pipeline) ListPlugins() []PluginInfo {
	infos := make([]PluginInfo, 0, len(p.plugins))
	for _, plugin := range p.plugins {
		info := PluginInfo{
			ID:      plugin.ID(),
			Type:    PluginTypeFeature,
			Enabled: true,
		}
		if cp, ok := plugin.(CorePlugin); ok && cp.IsCore() {
			info.Type = PluginTypeCore
		}
		if cp, ok := plugin.(ConfigurablePlugin); ok {
			info.Config = cp.Config()
		}
		infos = append(infos, info)
	}
	return infos
}

// ProcessHTTP runs hooks in order: PreStore → Storage → PostStore → HTTPResponse.
func (p *Pipeline) ProcessHTTP(ctx context.Context, e *events.HTTPEvent) error {
	for _, hook := range p.preStore {
		if err := hook.OnPreStore(ctx, &e.Event); err != nil {
			p.logger.Warn("prestore hook error",
				zap.String("plugin", pluginID(hook)),
				zap.Error(err))
		}
	}

	if !e.Draft.Drop && p.store != nil {
		id, err := p.store.CreateInteraction(ctx, e.Draft)
		if err != nil {
			return err
		}
		e.InteractionID = id

		if len(e.Draft.Attributes) > 0 {
			if err := p.store.SaveAttributes(ctx, id, e.Draft.Attributes); err != nil {
				p.logger.Warn("failed to save attributes", zap.Error(err))
			}
		}
	}

	for _, hook := range p.postStore {
		if err := hook.OnPostStore(ctx, &e.Event); err != nil {
			p.logger.Warn("poststore hook error",
				zap.String("plugin", pluginID(hook)),
				zap.Error(err))
		}
	}

	for _, hook := range p.httpResponse {
		if err := hook.OnHTTPResponse(ctx, e); err != nil {
			p.logger.Warn("http response hook error",
				zap.String("plugin", pluginID(hook)),
				zap.Error(err))
		}
		if e.Resp != nil && e.Resp.Handled {
			break
		}
	}

	return nil
}

// ProcessDNS runs hooks in order: PreStore → Storage → PostStore → DNSResponse.
func (p *Pipeline) ProcessDNS(ctx context.Context, e *events.DNSEvent) error {
	for _, hook := range p.preStore {
		if err := hook.OnPreStore(ctx, &e.Event); err != nil {
			p.logger.Warn("prestore hook error",
				zap.String("plugin", pluginID(hook)),
				zap.Error(err))
		}
	}

	if !e.Draft.Drop && p.store != nil {
		id, err := p.store.CreateInteraction(ctx, e.Draft)
		if err != nil {
			return err
		}
		e.InteractionID = id

		if len(e.Draft.Attributes) > 0 {
			if err := p.store.SaveAttributes(ctx, id, e.Draft.Attributes); err != nil {
				p.logger.Warn("failed to save attributes", zap.Error(err))
			}
		}
	}

	for _, hook := range p.postStore {
		if err := hook.OnPostStore(ctx, &e.Event); err != nil {
			p.logger.Warn("poststore hook error",
				zap.String("plugin", pluginID(hook)),
				zap.Error(err))
		}
	}

	for _, hook := range p.dnsResponse {
		if err := hook.OnDNSResponse(ctx, e); err != nil {
			p.logger.Warn("dns response hook error",
				zap.String("plugin", pluginID(hook)),
				zap.Error(err))
		}
		if e.Resp != nil && e.Resp.Handled {
			break
		}
	}

	return nil
}

func pluginID(hook any) string {
	if p, ok := hook.(Plugin); ok {
		return p.ID()
	}
	return "unknown"
}
