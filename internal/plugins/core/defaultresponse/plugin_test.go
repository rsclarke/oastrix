package defaultresponse

import (
	"context"
	"testing"

	"github.com/miekg/dns"
	"go.uber.org/zap"

	"github.com/rsclarke/oastrix/internal/events"
	"github.com/rsclarke/oastrix/internal/plugins"
)

func TestPluginID(t *testing.T) {
	p := New("1.2.3.4")
	if got := p.ID(); got != "defaultresponse" {
		t.Errorf("ID() = %q, want %q", got, "defaultresponse")
	}
}

func TestPluginInit(t *testing.T) {
	p := New("1.2.3.4")
	err := p.Init(plugins.InitContext{Logger: zap.NewNop()})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if p.logger == nil {
		t.Error("expected logger to be set after Init")
	}
}

func TestPluginPriority(t *testing.T) {
	p := New("1.2.3.4")
	if got := p.Priority(); got != 999 {
		t.Errorf("Priority() = %d, want 999", got)
	}
}

func TestOnHTTPResponseSetsDefault(t *testing.T) {
	p := New("1.2.3.4")
	_ = p.Init(plugins.InitContext{Logger: zap.NewNop()})

	e := &events.HTTPEvent{
		Resp: &events.HTTPResponsePlan{
			Handled: false,
		},
	}

	err := p.OnHTTPResponse(context.Background(), e)
	if err != nil {
		t.Fatalf("OnHTTPResponse failed: %v", err)
	}

	if e.Resp.Status != 200 {
		t.Errorf("Status = %d, want 200", e.Resp.Status)
	}
	if string(e.Resp.Body) != "ok" {
		t.Errorf("Body = %q, want %q", string(e.Resp.Body), "ok")
	}
	if !e.Resp.Handled {
		t.Error("expected Handled to be true")
	}
}

func TestOnHTTPResponseSkipsWhenAlreadyHandled(t *testing.T) {
	p := New("1.2.3.4")
	_ = p.Init(plugins.InitContext{Logger: zap.NewNop()})

	e := &events.HTTPEvent{
		Resp: &events.HTTPResponsePlan{
			Status:  418,
			Body:    []byte("I'm a teapot"),
			Handled: true,
		},
	}

	err := p.OnHTTPResponse(context.Background(), e)
	if err != nil {
		t.Fatalf("OnHTTPResponse failed: %v", err)
	}

	if e.Resp.Status != 418 {
		t.Errorf("Status = %d, want 418 (unchanged)", e.Resp.Status)
	}
	if string(e.Resp.Body) != "I'm a teapot" {
		t.Errorf("Body = %q, want %q", string(e.Resp.Body), "I'm a teapot")
	}
}

func TestOnHTTPResponseSkipsNilResp(t *testing.T) {
	p := New("1.2.3.4")
	_ = p.Init(plugins.InitContext{Logger: zap.NewNop()})

	e := &events.HTTPEvent{
		Resp: nil,
	}

	err := p.OnHTTPResponse(context.Background(), e)
	if err != nil {
		t.Fatalf("OnHTTPResponse failed: %v", err)
	}

	if e.Resp != nil {
		t.Error("expected Resp to remain nil")
	}
}

func TestOnDNSResponseSetsARecord(t *testing.T) {
	p := New("1.2.3.4")
	_ = p.Init(plugins.InitContext{Logger: zap.NewNop()})

	e := &events.DNSEvent{
		Event: events.Event{
			Draft: &events.InteractionDraft{
				DNS: &events.DNSDraft{
					QName: "test.example.com",
					QType: int(dns.TypeA),
				},
			},
		},
		Resp: &events.DNSResponsePlan{
			Handled: false,
		},
	}

	err := p.OnDNSResponse(context.Background(), e)
	if err != nil {
		t.Fatalf("OnDNSResponse failed: %v", err)
	}

	if len(e.Resp.Answers) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(e.Resp.Answers))
	}

	aRecord, ok := e.Resp.Answers[0].(*dns.A)
	if !ok {
		t.Fatalf("expected A record, got %T", e.Resp.Answers[0])
	}

	if aRecord.A.String() != "1.2.3.4" {
		t.Errorf("A record IP = %q, want %q", aRecord.A.String(), "1.2.3.4")
	}

	if aRecord.Hdr.Name != "test.example.com." {
		t.Errorf("A record name = %q, want %q", aRecord.Hdr.Name, "test.example.com.")
	}

	if !e.Resp.Handled {
		t.Error("expected Handled to be true")
	}
}

func TestOnDNSResponseSkipsWhenAlreadyHandled(t *testing.T) {
	p := New("1.2.3.4")
	_ = p.Init(plugins.InitContext{Logger: zap.NewNop()})

	existingRR := &dns.A{
		Hdr: dns.RR_Header{
			Name:   "existing.example.com.",
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    600,
		},
		A: []byte{5, 6, 7, 8},
	}

	e := &events.DNSEvent{
		Event: events.Event{
			Draft: &events.InteractionDraft{
				DNS: &events.DNSDraft{
					QName: "test.example.com",
					QType: int(dns.TypeA),
				},
			},
		},
		Resp: &events.DNSResponsePlan{
			Answers: []dns.RR{existingRR},
			Handled: true,
		},
	}

	err := p.OnDNSResponse(context.Background(), e)
	if err != nil {
		t.Fatalf("OnDNSResponse failed: %v", err)
	}

	if len(e.Resp.Answers) != 1 {
		t.Errorf("expected 1 answer (unchanged), got %d", len(e.Resp.Answers))
	}
}

func TestOnDNSResponseSkipsNonARecord(t *testing.T) {
	p := New("1.2.3.4")
	_ = p.Init(plugins.InitContext{Logger: zap.NewNop()})

	e := &events.DNSEvent{
		Event: events.Event{
			Draft: &events.InteractionDraft{
				DNS: &events.DNSDraft{
					QName: "test.example.com",
					QType: int(dns.TypeAAAA),
				},
			},
		},
		Resp: &events.DNSResponsePlan{
			Handled: false,
		},
	}

	err := p.OnDNSResponse(context.Background(), e)
	if err != nil {
		t.Fatalf("OnDNSResponse failed: %v", err)
	}

	if len(e.Resp.Answers) != 0 {
		t.Errorf("expected no answers for non-A query, got %d", len(e.Resp.Answers))
	}
	if e.Resp.Handled {
		t.Error("expected Handled to remain false for non-A query")
	}
}

func TestOnDNSResponseSkipsNilResp(t *testing.T) {
	p := New("1.2.3.4")
	_ = p.Init(plugins.InitContext{Logger: zap.NewNop()})

	e := &events.DNSEvent{
		Event: events.Event{
			Draft: &events.InteractionDraft{
				DNS: &events.DNSDraft{
					QName: "test.example.com",
					QType: int(dns.TypeA),
				},
			},
		},
		Resp: nil,
	}

	err := p.OnDNSResponse(context.Background(), e)
	if err != nil {
		t.Fatalf("OnDNSResponse failed: %v", err)
	}

	if e.Resp != nil {
		t.Error("expected Resp to remain nil")
	}
}

func TestOnDNSResponseSkipsNilDraft(t *testing.T) {
	p := New("1.2.3.4")
	_ = p.Init(plugins.InitContext{Logger: zap.NewNop()})

	e := &events.DNSEvent{
		Event: events.Event{
			Draft: nil,
		},
		Resp: &events.DNSResponsePlan{
			Handled: false,
		},
	}

	err := p.OnDNSResponse(context.Background(), e)
	if err != nil {
		t.Fatalf("OnDNSResponse failed: %v", err)
	}

	if len(e.Resp.Answers) != 0 {
		t.Errorf("expected no answers when Draft is nil, got %d", len(e.Resp.Answers))
	}
}

func TestOnDNSResponseSkipsNilDNSDraft(t *testing.T) {
	p := New("1.2.3.4")
	_ = p.Init(plugins.InitContext{Logger: zap.NewNop()})

	e := &events.DNSEvent{
		Event: events.Event{
			Draft: &events.InteractionDraft{
				DNS: nil,
			},
		},
		Resp: &events.DNSResponsePlan{
			Handled: false,
		},
	}

	err := p.OnDNSResponse(context.Background(), e)
	if err != nil {
		t.Fatalf("OnDNSResponse failed: %v", err)
	}

	if len(e.Resp.Answers) != 0 {
		t.Errorf("expected no answers when DNS is nil, got %d", len(e.Resp.Answers))
	}
}

func TestNewWithInvalidIP(t *testing.T) {
	p := New("not-a-valid-ip")
	if p.publicIP.String() != "127.0.0.1" {
		t.Errorf("publicIP = %q, want %q for invalid input", p.publicIP.String(), "127.0.0.1")
	}
}

func TestNewWithValidIP(t *testing.T) {
	p := New("10.20.30.40")
	if p.publicIP.String() != "10.20.30.40" {
		t.Errorf("publicIP = %q, want %q", p.publicIP.String(), "10.20.30.40")
	}
}

func TestOnDNSResponsePreservesTrailingDot(t *testing.T) {
	p := New("1.2.3.4")
	_ = p.Init(plugins.InitContext{Logger: zap.NewNop()})

	e := &events.DNSEvent{
		Event: events.Event{
			Draft: &events.InteractionDraft{
				DNS: &events.DNSDraft{
					QName: "test.example.com.",
					QType: int(dns.TypeA),
				},
			},
		},
		Resp: &events.DNSResponsePlan{
			Handled: false,
		},
	}

	err := p.OnDNSResponse(context.Background(), e)
	if err != nil {
		t.Fatalf("OnDNSResponse failed: %v", err)
	}

	aRecord := e.Resp.Answers[0].(*dns.A)
	if aRecord.Hdr.Name != "test.example.com." {
		t.Errorf("A record name = %q, want %q", aRecord.Hdr.Name, "test.example.com.")
	}
}
