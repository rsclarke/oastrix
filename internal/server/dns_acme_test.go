package server

import (
	"net"
	"testing"

	"github.com/miekg/dns"
	"github.com/rsclarke/oastrix/internal/acme"
)

type mockResponseWriter struct {
	msg        *dns.Msg
	remoteAddr net.Addr
}

func (m *mockResponseWriter) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 53}
}

func (m *mockResponseWriter) RemoteAddr() net.Addr {
	return m.remoteAddr
}

func (m *mockResponseWriter) WriteMsg(msg *dns.Msg) error {
	m.msg = msg
	return nil
}

func (m *mockResponseWriter) Write([]byte) (int, error) {
	return 0, nil
}

func (m *mockResponseWriter) Close() error {
	return nil
}

func (m *mockResponseWriter) TsigStatus() error {
	return nil
}

func (m *mockResponseWriter) TsigTimersOnly(bool) {}

func (m *mockResponseWriter) Hijack() {}

func TestDNSServer_TXTQuery(t *testing.T) {
	store := acme.NewTXTStore()
	store.Add("_acme-challenge.example.com", "test-challenge-value")

	s := &DNSServer{
		Domain:   "example.com",
		TXTStore: store,
	}

	req := new(dns.Msg)
	req.SetQuestion("_acme-challenge.example.com.", dns.TypeTXT)

	w := &mockResponseWriter{
		remoteAddr: &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 12345},
	}

	s.handleDNS(w, req)

	if w.msg == nil {
		t.Fatal("expected response message")
	}

	if len(w.msg.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(w.msg.Answer))
	}

	txtRR, ok := w.msg.Answer[0].(*dns.TXT)
	if !ok {
		t.Fatalf("expected TXT record, got %T", w.msg.Answer[0])
	}

	if len(txtRR.Txt) != 1 || txtRR.Txt[0] != "test-challenge-value" {
		t.Errorf("expected TXT value 'test-challenge-value', got %v", txtRR.Txt)
	}
}

func TestDNSServer_TXTQueryMultipleValues(t *testing.T) {
	store := acme.NewTXTStore()
	store.Add("_acme-challenge.example.com", "value1")
	store.Add("_acme-challenge.example.com", "value2")

	s := &DNSServer{
		Domain:   "example.com",
		TXTStore: store,
	}

	req := new(dns.Msg)
	req.SetQuestion("_acme-challenge.example.com.", dns.TypeTXT)

	w := &mockResponseWriter{
		remoteAddr: &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 12345},
	}

	s.handleDNS(w, req)

	if w.msg == nil {
		t.Fatal("expected response message")
	}

	if len(w.msg.Answer) != 2 {
		t.Fatalf("expected 2 answers, got %d", len(w.msg.Answer))
	}

	values := make(map[string]bool)
	for _, ans := range w.msg.Answer {
		txtRR, ok := ans.(*dns.TXT)
		if !ok {
			t.Fatalf("expected TXT record, got %T", ans)
		}
		for _, v := range txtRR.Txt {
			values[v] = true
		}
	}

	if !values["value1"] || !values["value2"] {
		t.Errorf("expected both value1 and value2, got %v", values)
	}
}

func TestDNSServer_TXTQueryNoValue(t *testing.T) {
	store := acme.NewTXTStore()

	s := &DNSServer{
		Domain:   "example.com",
		TXTStore: store,
	}

	req := new(dns.Msg)
	req.SetQuestion("_acme-challenge.other.com.", dns.TypeTXT)

	w := &mockResponseWriter{
		remoteAddr: &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 12345},
	}

	s.handleDNS(w, req)

	if w.msg == nil {
		t.Fatal("expected response message")
	}

	if w.msg.Rcode != dns.RcodeNameError {
		t.Errorf("expected NXDOMAIN, got %d", w.msg.Rcode)
	}
}

func TestDNSServer_TXTQueryNoStore(t *testing.T) {
	s := &DNSServer{
		Domain:   "example.com",
		TXTStore: nil,
	}

	req := new(dns.Msg)
	req.SetQuestion("_acme-challenge.other.com.", dns.TypeTXT)

	w := &mockResponseWriter{
		remoteAddr: &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 12345},
	}

	s.handleDNS(w, req)

	if w.msg == nil {
		t.Fatal("expected response message")
	}

	if w.msg.Rcode != dns.RcodeNameError {
		t.Errorf("expected NXDOMAIN (no store, no token), got %d", w.msg.Rcode)
	}
}
