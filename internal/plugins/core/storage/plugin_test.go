package storage

import (
	"context"
	"database/sql"
	"testing"

	"go.uber.org/zap"

	"github.com/rsclarke/oastrix/internal/db"
	"github.com/rsclarke/oastrix/internal/events"
	"github.com/rsclarke/oastrix/internal/plugins"
)

func setupTestDB(t *testing.T) *sql.DB {
	tmpDB := t.TempDir() + "/test.db"
	database, err := db.Open(tmpDB)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})
	return database
}

func TestPluginID(t *testing.T) {
	p := New(nil)
	if got := p.ID(); got != "storage" {
		t.Errorf("ID() = %q, want %q", got, "storage")
	}
}

func TestPluginInit(t *testing.T) {
	p := New(nil)
	err := p.Init(plugins.InitContext{Logger: zap.NewNop()})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if p.logger == nil {
		t.Error("expected logger to be set after Init")
	}
}

func TestOnPreStoreResolvesToken(t *testing.T) {
	database := setupTestDB(t)
	p := New(database)
	_ = p.Init(plugins.InitContext{Logger: zap.NewNop()})

	tokenID, err := db.CreateToken(database, "test-token", nil, nil)
	if err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}

	e := &events.Event{
		Draft: &events.InteractionDraft{
			TokenValue: "test-token",
		},
	}

	err = p.OnPreStore(context.Background(), e)
	if err != nil {
		t.Fatalf("OnPreStore failed: %v", err)
	}

	if e.Draft.TokenID != tokenID {
		t.Errorf("TokenID = %d, want %d", e.Draft.TokenID, tokenID)
	}
}

func TestOnPreStoreSkipsWhenTokenIDSet(t *testing.T) {
	database := setupTestDB(t)
	p := New(database)
	_ = p.Init(plugins.InitContext{Logger: zap.NewNop()})

	e := &events.Event{
		Draft: &events.InteractionDraft{
			TokenValue: "test-token",
			TokenID:    42,
		},
	}

	err := p.OnPreStore(context.Background(), e)
	if err != nil {
		t.Fatalf("OnPreStore failed: %v", err)
	}

	if e.Draft.TokenID != 42 {
		t.Errorf("TokenID = %d, want 42 (unchanged)", e.Draft.TokenID)
	}
}

func TestOnPreStoreSkipsEmptyTokenValue(t *testing.T) {
	p := New(nil)
	_ = p.Init(plugins.InitContext{Logger: zap.NewNop()})

	e := &events.Event{
		Draft: &events.InteractionDraft{
			TokenValue: "",
		},
	}

	err := p.OnPreStore(context.Background(), e)
	if err != nil {
		t.Fatalf("OnPreStore failed: %v", err)
	}

	if e.Draft.TokenID != 0 {
		t.Errorf("TokenID = %d, want 0", e.Draft.TokenID)
	}
}

func TestOnPreStoreUnknownToken(t *testing.T) {
	database := setupTestDB(t)
	p := New(database)
	_ = p.Init(plugins.InitContext{Logger: zap.NewNop()})

	e := &events.Event{
		Draft: &events.InteractionDraft{
			TokenValue: "nonexistent-token",
		},
	}

	err := p.OnPreStore(context.Background(), e)
	if err != nil {
		t.Fatalf("OnPreStore failed: %v", err)
	}

	if e.Draft.TokenID != 0 {
		t.Errorf("TokenID = %d, want 0 for unknown token", e.Draft.TokenID)
	}
}

func TestResolveTokenID(t *testing.T) {
	database := setupTestDB(t)
	p := New(database)
	_ = p.Init(plugins.InitContext{Logger: zap.NewNop()})

	tokenID, err := db.CreateToken(database, "test-token", nil, nil)
	if err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}

	id, found, err := p.ResolveTokenID(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("ResolveTokenID failed: %v", err)
	}
	if !found {
		t.Error("expected token to be found")
	}
	if id != tokenID {
		t.Errorf("ID = %d, want %d", id, tokenID)
	}

	id, found, err = p.ResolveTokenID(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("ResolveTokenID failed: %v", err)
	}
	if found {
		t.Error("expected token not to be found")
	}
	if id != 0 {
		t.Errorf("ID = %d, want 0", id)
	}
}

func TestCreateInteractionSkipsZeroTokenID(t *testing.T) {
	database := setupTestDB(t)
	p := New(database)
	_ = p.Init(plugins.InitContext{Logger: zap.NewNop()})

	draft := &events.InteractionDraft{
		TokenID:    0,
		Kind:       events.KindHTTP,
		RemoteIP:   "192.168.1.1",
		RemotePort: 54321,
		TLS:        false,
		Summary:    "GET /",
	}

	id, err := p.CreateInteraction(context.Background(), draft)
	if err != nil {
		t.Fatalf("CreateInteraction failed: %v", err)
	}
	if id != 0 {
		t.Errorf("expected 0 ID for zero TokenID, got %d", id)
	}
}

func TestStoreHTTPInteraction(t *testing.T) {
	database := setupTestDB(t)
	p := New(database)
	_ = p.Init(plugins.InitContext{Logger: zap.NewNop()})

	tokenID, err := db.CreateToken(database, "test-token", nil, nil)
	if err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}

	draft := &events.InteractionDraft{
		TokenID:    tokenID,
		Kind:       events.KindHTTP,
		RemoteIP:   "192.168.1.1",
		RemotePort: 54321,
		TLS:        true,
		Summary:    "GET /test",
		HTTP: &events.HTTPDraft{
			Method:  "GET",
			Scheme:  "https",
			Host:    "example.com",
			Path:    "/test",
			Query:   "foo=bar",
			Proto:   "HTTP/1.1",
			Headers: map[string][]string{"User-Agent": {"test-agent"}},
			Body:    []byte("test body"),
		},
	}

	id, err := p.CreateInteraction(context.Background(), draft)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	if id == 0 {
		t.Error("expected non-zero interaction ID")
	}

	interactions, err := db.GetInteractionsByToken(database, tokenID)
	if err != nil {
		t.Fatalf("GetInteractionsByToken failed: %v", err)
	}
	if len(interactions) != 1 {
		t.Fatalf("expected 1 interaction, got %d", len(interactions))
	}

	httpInteraction, err := db.GetHTTPInteraction(database, id)
	if err != nil {
		t.Fatalf("GetHTTPInteraction failed: %v", err)
	}
	if httpInteraction == nil {
		t.Fatal("expected HTTP interaction to exist")
	}
	if httpInteraction.Method != "GET" {
		t.Errorf("Method = %q, want %q", httpInteraction.Method, "GET")
	}
	if httpInteraction.Path != "/test" {
		t.Errorf("Path = %q, want %q", httpInteraction.Path, "/test")
	}
}

func TestStoreDNSInteraction(t *testing.T) {
	database := setupTestDB(t)
	p := New(database)
	_ = p.Init(plugins.InitContext{Logger: zap.NewNop()})

	tokenID, err := db.CreateToken(database, "test-token", nil, nil)
	if err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}

	draft := &events.InteractionDraft{
		TokenID:    tokenID,
		Kind:       events.KindDNS,
		RemoteIP:   "192.168.1.1",
		RemotePort: 53,
		TLS:        false,
		Summary:    "A test.example.com",
		DNS: &events.DNSDraft{
			QName:    "test.example.com",
			QType:    1,
			QClass:   1,
			RD:       1,
			Opcode:   0,
			DNSID:    12345,
			Protocol: "udp",
		},
	}

	id, err := p.CreateInteraction(context.Background(), draft)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	if id == 0 {
		t.Error("expected non-zero interaction ID")
	}

	dnsInteraction, err := db.GetDNSInteraction(database, id)
	if err != nil {
		t.Fatalf("GetDNSInteraction failed: %v", err)
	}
	if dnsInteraction == nil {
		t.Fatal("expected DNS interaction to exist")
	}
	if dnsInteraction.QName != "test.example.com" {
		t.Errorf("QName = %q, want %q", dnsInteraction.QName, "test.example.com")
	}
	if dnsInteraction.DNSID != 12345 {
		t.Errorf("DNSID = %d, want %d", dnsInteraction.DNSID, 12345)
	}
}

func TestSaveAttributes(t *testing.T) {
	database := setupTestDB(t)
	p := New(database)
	_ = p.Init(plugins.InitContext{Logger: zap.NewNop()})

	tokenID, err := db.CreateToken(database, "test-token", nil, nil)
	if err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}

	draft := &events.InteractionDraft{
		TokenID:    tokenID,
		Kind:       events.KindHTTP,
		RemoteIP:   "192.168.1.1",
		RemotePort: 54321,
		TLS:        false,
		Summary:    "GET /",
		HTTP: &events.HTTPDraft{
			Method:  "GET",
			Scheme:  "http",
			Host:    "example.com",
			Path:    "/",
			Query:   "",
			Proto:   "HTTP/1.1",
			Headers: map[string][]string{},
			Body:    nil,
		},
	}

	id, err := p.CreateInteraction(context.Background(), draft)
	if err != nil {
		t.Fatalf("CreateInteraction failed: %v", err)
	}

	attrs := map[string]any{
		"plugin_data":  "test-value",
		"numeric_data": 42,
	}
	err = p.SaveAttributes(context.Background(), id, attrs)
	if err != nil {
		t.Fatalf("SaveAttributes failed: %v", err)
	}

	savedAttrs, err := db.GetAttributes(database, id)
	if err != nil {
		t.Fatalf("GetAttributes failed: %v", err)
	}

	if savedAttrs["plugin_data"] != "test-value" {
		t.Errorf("plugin_data = %v, want %q", savedAttrs["plugin_data"], "test-value")
	}
	if v, ok := savedAttrs["numeric_data"].(float64); !ok || v != 42 {
		t.Errorf("numeric_data = %v, want 42", savedAttrs["numeric_data"])
	}
}

func TestStoreHTTPWithoutHTTPDraft(t *testing.T) {
	database := setupTestDB(t)
	p := New(database)
	_ = p.Init(plugins.InitContext{Logger: zap.NewNop()})

	tokenID, err := db.CreateToken(database, "test-token", nil, nil)
	if err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}

	draft := &events.InteractionDraft{
		TokenID:    tokenID,
		Kind:       events.KindHTTP,
		RemoteIP:   "192.168.1.1",
		RemotePort: 54321,
		TLS:        false,
		Summary:    "GET /",
		HTTP:       nil,
	}

	id, err := p.CreateInteraction(context.Background(), draft)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	if id == 0 {
		t.Error("expected non-zero interaction ID")
	}

	httpInteraction, err := db.GetHTTPInteraction(database, id)
	if err != nil {
		t.Fatalf("GetHTTPInteraction failed: %v", err)
	}
	if httpInteraction != nil {
		t.Error("expected no HTTP interaction when HTTPDraft is nil")
	}
}

func TestStoreDNSWithoutDNSDraft(t *testing.T) {
	database := setupTestDB(t)
	p := New(database)
	_ = p.Init(plugins.InitContext{Logger: zap.NewNop()})

	tokenID, err := db.CreateToken(database, "test-token", nil, nil)
	if err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}

	draft := &events.InteractionDraft{
		TokenID:    tokenID,
		Kind:       events.KindDNS,
		RemoteIP:   "192.168.1.1",
		RemotePort: 53,
		TLS:        false,
		Summary:    "A query",
		DNS:        nil,
	}

	id, err := p.CreateInteraction(context.Background(), draft)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	if id == 0 {
		t.Error("expected non-zero interaction ID")
	}

	dnsInteraction, err := db.GetDNSInteraction(database, id)
	if err != nil {
		t.Fatalf("GetDNSInteraction failed: %v", err)
	}
	if dnsInteraction != nil {
		t.Error("expected no DNS interaction when DNSDraft is nil")
	}
}
