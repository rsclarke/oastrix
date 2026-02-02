package plugins

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"

	"github.com/rsclarke/oastrix/internal/events"
)

type mockStore struct {
	createCalled    bool
	saveCalled      bool
	createErr       error
	saveErr         error
	lastDraft       *events.InteractionDraft
	lastInteraction int64
	lastAttrs       map[string]any
	returnedID      int64
}

func (m *mockStore) ResolveTokenID(_ context.Context, _ string) (int64, bool, error) {
	return 1, true, nil
}

func (m *mockStore) CreateInteraction(_ context.Context, draft *events.InteractionDraft) (int64, error) {
	m.createCalled = true
	m.lastDraft = draft
	if m.createErr != nil {
		return 0, m.createErr
	}
	return m.returnedID, nil
}

func (m *mockStore) SaveAttributes(_ context.Context, id int64, attrs map[string]any) error {
	m.saveCalled = true
	m.lastInteraction = id
	m.lastAttrs = attrs
	return m.saveErr
}

type callRecord struct {
	pluginID string
	phase    string
}

type mockPlugin struct {
	id         string
	preErr     error
	postErr    error
	httpErr    error
	dnsErr     error
	calls      *[]callRecord
	setHandled bool
}

func (m *mockPlugin) ID() string { return m.id }

func (m *mockPlugin) Init(_ InitContext) error { return nil }

func (m *mockPlugin) OnPreStore(_ context.Context, _ *events.Event) error {
	if m.calls != nil {
		*m.calls = append(*m.calls, callRecord{m.id, "prestore"})
	}
	return m.preErr
}

func (m *mockPlugin) OnPostStore(_ context.Context, _ *events.Event) error {
	if m.calls != nil {
		*m.calls = append(*m.calls, callRecord{m.id, "poststore"})
	}
	return m.postErr
}

func (m *mockPlugin) OnHTTPResponse(_ context.Context, e *events.HTTPEvent) error {
	if m.calls != nil {
		*m.calls = append(*m.calls, callRecord{m.id, "httpresponse"})
	}
	if m.setHandled && e.Resp != nil {
		e.Resp.Handled = true
	}
	return m.httpErr
}

func (m *mockPlugin) OnDNSResponse(_ context.Context, e *events.DNSEvent) error {
	if m.calls != nil {
		*m.calls = append(*m.calls, callRecord{m.id, "dnsresponse"})
	}
	if m.setHandled && e.Resp != nil {
		e.Resp.Handled = true
	}
	return m.dnsErr
}

func TestNewPipeline(t *testing.T) {
	logger := zap.NewNop()
	p := NewPipeline(logger)
	if p == nil {
		t.Fatal("expected non-nil pipeline")
	}
	if p.logger != logger {
		t.Error("expected logger to be set")
	}
}

func TestRegisterDetectsCapabilities(t *testing.T) {
	p := NewPipeline(zap.NewNop())
	plugin := &mockPlugin{id: "test"}

	p.Register(plugin)

	if len(p.preStore) != 1 {
		t.Errorf("expected 1 prestore hook, got %d", len(p.preStore))
	}
	if len(p.postStore) != 1 {
		t.Errorf("expected 1 poststore hook, got %d", len(p.postStore))
	}
	if len(p.httpResponse) != 1 {
		t.Errorf("expected 1 http response hook, got %d", len(p.httpResponse))
	}
	if len(p.dnsResponse) != 1 {
		t.Errorf("expected 1 dns response hook, got %d", len(p.dnsResponse))
	}
}

func TestProcessHTTPHookOrdering(t *testing.T) {
	var calls []callRecord
	p := NewPipeline(zap.NewNop())
	store := &mockStore{returnedID: 42}
	p.SetStore(store)

	p.Register(&mockPlugin{id: "p1", calls: &calls})
	p.Register(&mockPlugin{id: "p2", calls: &calls})

	e := &events.HTTPEvent{
		Event: events.Event{
			Draft: &events.InteractionDraft{TokenValue: "test"},
		},
		Resp: &events.HTTPResponsePlan{},
	}

	err := p.ProcessHTTP(context.Background(), e)
	if err != nil {
		t.Fatalf("ProcessHTTP failed: %v", err)
	}

	expected := []callRecord{
		{"p1", "prestore"},
		{"p2", "prestore"},
		{"p1", "poststore"},
		{"p2", "poststore"},
		{"p1", "httpresponse"},
		{"p2", "httpresponse"},
	}

	if len(calls) != len(expected) {
		t.Fatalf("expected %d calls, got %d: %v", len(expected), len(calls), calls)
	}
	for i, exp := range expected {
		if calls[i] != exp {
			t.Errorf("call %d: expected %v, got %v", i, exp, calls[i])
		}
	}

	if e.InteractionID != 42 {
		t.Errorf("expected InteractionID 42, got %d", e.InteractionID)
	}
}

func TestProcessDNSHookOrdering(t *testing.T) {
	var calls []callRecord
	p := NewPipeline(zap.NewNop())
	store := &mockStore{returnedID: 99}
	p.SetStore(store)

	p.Register(&mockPlugin{id: "p1", calls: &calls})
	p.Register(&mockPlugin{id: "p2", calls: &calls})

	e := &events.DNSEvent{
		Event: events.Event{
			Draft: &events.InteractionDraft{TokenValue: "test"},
		},
		Resp: &events.DNSResponsePlan{},
	}

	err := p.ProcessDNS(context.Background(), e)
	if err != nil {
		t.Fatalf("ProcessDNS failed: %v", err)
	}

	expected := []callRecord{
		{"p1", "prestore"},
		{"p2", "prestore"},
		{"p1", "poststore"},
		{"p2", "poststore"},
		{"p1", "dnsresponse"},
		{"p2", "dnsresponse"},
	}

	if len(calls) != len(expected) {
		t.Fatalf("expected %d calls, got %d: %v", len(expected), len(calls), calls)
	}
	for i, exp := range expected {
		if calls[i] != exp {
			t.Errorf("call %d: expected %v, got %v", i, exp, calls[i])
		}
	}

	if e.InteractionID != 99 {
		t.Errorf("expected InteractionID 99, got %d", e.InteractionID)
	}
}

func TestDropSkipsStorage(t *testing.T) {
	var calls []callRecord
	p := NewPipeline(zap.NewNop())
	store := &mockStore{returnedID: 42}
	p.SetStore(store)

	p.Register(&mockPlugin{id: "p1", calls: &calls})

	e := &events.HTTPEvent{
		Event: events.Event{
			Draft: &events.InteractionDraft{
				TokenValue: "test",
				Drop:       true,
			},
		},
		Resp: &events.HTTPResponsePlan{},
	}

	err := p.ProcessHTTP(context.Background(), e)
	if err != nil {
		t.Fatalf("ProcessHTTP failed: %v", err)
	}

	if store.createCalled {
		t.Error("expected storage to be skipped when Drop is true")
	}

	if e.InteractionID != 0 {
		t.Errorf("expected InteractionID 0 when dropped, got %d", e.InteractionID)
	}

	expected := []callRecord{
		{"p1", "prestore"},
		{"p1", "poststore"},
		{"p1", "httpresponse"},
	}
	if len(calls) != len(expected) {
		t.Fatalf("expected %d calls, got %d", len(expected), len(calls))
	}
}

func TestDropSkipsStorageForDNS(t *testing.T) {
	p := NewPipeline(zap.NewNop())
	store := &mockStore{returnedID: 42}
	p.SetStore(store)

	e := &events.DNSEvent{
		Event: events.Event{
			Draft: &events.InteractionDraft{
				TokenValue: "test",
				Drop:       true,
			},
		},
		Resp: &events.DNSResponsePlan{},
	}

	err := p.ProcessDNS(context.Background(), e)
	if err != nil {
		t.Fatalf("ProcessDNS failed: %v", err)
	}

	if store.createCalled {
		t.Error("expected storage to be skipped when Drop is true")
	}
}

func TestHandledStopsHTTPResponseHooks(t *testing.T) {
	var calls []callRecord
	p := NewPipeline(zap.NewNop())
	store := &mockStore{returnedID: 42}
	p.SetStore(store)

	p.Register(&mockPlugin{id: "p1", calls: &calls, setHandled: true})
	p.Register(&mockPlugin{id: "p2", calls: &calls})

	e := &events.HTTPEvent{
		Event: events.Event{
			Draft: &events.InteractionDraft{TokenValue: "test"},
		},
		Resp: &events.HTTPResponsePlan{},
	}

	err := p.ProcessHTTP(context.Background(), e)
	if err != nil {
		t.Fatalf("ProcessHTTP failed: %v", err)
	}

	expected := []callRecord{
		{"p1", "prestore"},
		{"p2", "prestore"},
		{"p1", "poststore"},
		{"p2", "poststore"},
		{"p1", "httpresponse"},
	}

	if len(calls) != len(expected) {
		t.Fatalf("expected %d calls, got %d: %v", len(expected), len(calls), calls)
	}
}

func TestHandledStopsDNSResponseHooks(t *testing.T) {
	var calls []callRecord
	p := NewPipeline(zap.NewNop())
	store := &mockStore{returnedID: 42}
	p.SetStore(store)

	p.Register(&mockPlugin{id: "p1", calls: &calls, setHandled: true})
	p.Register(&mockPlugin{id: "p2", calls: &calls})

	e := &events.DNSEvent{
		Event: events.Event{
			Draft: &events.InteractionDraft{TokenValue: "test"},
		},
		Resp: &events.DNSResponsePlan{},
	}

	err := p.ProcessDNS(context.Background(), e)
	if err != nil {
		t.Fatalf("ProcessDNS failed: %v", err)
	}

	expected := []callRecord{
		{"p1", "prestore"},
		{"p2", "prestore"},
		{"p1", "poststore"},
		{"p2", "poststore"},
		{"p1", "dnsresponse"},
	}

	if len(calls) != len(expected) {
		t.Fatalf("expected %d calls, got %d: %v", len(expected), len(calls), calls)
	}
}

func TestHookErrorsAreLoggedButDontStopPipeline(t *testing.T) {
	var calls []callRecord
	p := NewPipeline(zap.NewNop())
	store := &mockStore{returnedID: 42}
	p.SetStore(store)

	p.Register(&mockPlugin{id: "p1", calls: &calls, preErr: errors.New("pre error")})
	p.Register(&mockPlugin{id: "p2", calls: &calls})

	e := &events.HTTPEvent{
		Event: events.Event{
			Draft: &events.InteractionDraft{TokenValue: "test"},
		},
		Resp: &events.HTTPResponsePlan{},
	}

	err := p.ProcessHTTP(context.Background(), e)
	if err != nil {
		t.Fatalf("ProcessHTTP should not fail on hook error: %v", err)
	}

	if len(calls) != 6 {
		t.Errorf("expected 6 calls despite error, got %d: %v", len(calls), calls)
	}
}

func TestStorageErrorReturnsError(t *testing.T) {
	p := NewPipeline(zap.NewNop())
	store := &mockStore{createErr: errors.New("storage failed")}
	p.SetStore(store)

	e := &events.HTTPEvent{
		Event: events.Event{
			Draft: &events.InteractionDraft{TokenValue: "test"},
		},
		Resp: &events.HTTPResponsePlan{},
	}

	err := p.ProcessHTTP(context.Background(), e)
	if err == nil {
		t.Fatal("expected error from storage failure")
	}
	if err.Error() != "storage failed" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAttributesSaved(t *testing.T) {
	p := NewPipeline(zap.NewNop())
	store := &mockStore{returnedID: 42}
	p.SetStore(store)

	attrs := map[string]any{"key": "value"}
	e := &events.HTTPEvent{
		Event: events.Event{
			Draft: &events.InteractionDraft{
				TokenValue: "test",
				Attributes: attrs,
			},
		},
		Resp: &events.HTTPResponsePlan{},
	}

	err := p.ProcessHTTP(context.Background(), e)
	if err != nil {
		t.Fatalf("ProcessHTTP failed: %v", err)
	}

	if !store.saveCalled {
		t.Error("expected SaveAttributes to be called")
	}
	if store.lastInteraction != 42 {
		t.Errorf("expected interaction ID 42, got %d", store.lastInteraction)
	}
	if store.lastAttrs["key"] != "value" {
		t.Error("expected attributes to be passed correctly")
	}
}

func TestNoStoreDoesNotPanic(t *testing.T) {
	p := NewPipeline(zap.NewNop())

	e := &events.HTTPEvent{
		Event: events.Event{
			Draft: &events.InteractionDraft{TokenValue: "test"},
		},
		Resp: &events.HTTPResponsePlan{},
	}

	err := p.ProcessHTTP(context.Background(), e)
	if err != nil {
		t.Fatalf("ProcessHTTP failed: %v", err)
	}
}

func TestPluginIDHelper(t *testing.T) {
	plugin := &mockPlugin{id: "test-plugin"}
	if id := pluginID(plugin); id != "test-plugin" {
		t.Errorf("expected 'test-plugin', got '%s'", id)
	}

	nonPlugin := struct{}{}
	if id := pluginID(nonPlugin); id != "unknown" {
		t.Errorf("expected 'unknown', got '%s'", id)
	}
}
