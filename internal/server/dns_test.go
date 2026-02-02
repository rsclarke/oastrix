package server

import (
	"net"
	"os"
	"testing"

	"github.com/miekg/dns"
	"github.com/rsclarke/oastrix/internal/db"
	"go.uber.org/zap"
)

func TestExtractTokenFromQName(t *testing.T) {
	tests := []struct {
		name     string
		qname    string
		domain   string
		expected string
	}{
		{
			name:     "simple subdomain",
			qname:    "abc123.oastrix.local",
			domain:   "oastrix.local",
			expected: "abc123",
		},
		{
			name:     "nested subdomain returns first part",
			qname:    "sub.abc123.oastrix.local",
			domain:   "oastrix.local",
			expected: "sub",
		},
		{
			name:     "exact domain match - no subdomain",
			qname:    "oastrix.local",
			domain:   "oastrix.local",
			expected: "",
		},
		{
			name:     "different domain",
			qname:    "other.domain.com",
			domain:   "oastrix.local",
			expected: "",
		},
		{
			name:     "uppercase domain already lowercased by handler",
			qname:    "abc123.oastrix.local",
			domain:   "oastrix.local",
			expected: "abc123",
		},
		{
			name:     "multiple subdomains",
			qname:    "a.b.c.oastrix.local",
			domain:   "oastrix.local",
			expected: "a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTokenFromQName(tt.qname, tt.domain)
			if got != tt.expected {
				t.Errorf("extractTokenFromQName(%q, %q) = %q, want %q", tt.qname, tt.domain, got, tt.expected)
			}
		})
	}
}

func TestDNSServer_StoresInteraction(t *testing.T) {
	tmpDB := t.TempDir() + "/test.db"
	database, err := db.Open(tmpDB)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer func() { _ = database.Close() }()
	defer func() { _ = os.Remove(tmpDB) }()

	tokenValue := "testtoken123"
	_, err = db.CreateToken(database, tokenValue, nil, nil)
	if err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	srv := &DNSServer{
		Pipeline: setupPipeline(t, database),
		Domain:   "oastrix.local",
		PublicIP: "127.0.0.1",
		Logger:   zap.NewNop(),
	}

	req := new(dns.Msg)
	req.SetQuestion("testtoken123.oastrix.local.", dns.TypeA)
	req.RecursionDesired = true

	w := &mockResponseWriter{remoteAddr: &net.UDPAddr{IP: net.ParseIP("192.0.2.1"), Port: 12345}}
	srv.handleDNS(w, req)

	if w.msg == nil {
		t.Fatal("expected response message")
	}
	if w.msg.Rcode != dns.RcodeSuccess {
		t.Errorf("expected RcodeSuccess, got %d", w.msg.Rcode)
	}
	if len(w.msg.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(w.msg.Answer))
	}
	aRecord, ok := w.msg.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("expected A record, got %T", w.msg.Answer[0])
	}
	if !aRecord.A.Equal(net.ParseIP("127.0.0.1")) {
		t.Errorf("expected 127.0.0.1, got %s", aRecord.A)
	}

	var count int
	err = database.QueryRow("SELECT COUNT(*) FROM interactions").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count interactions: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 interaction, got %d", count)
	}

	var qname, protocol string
	var qtype int
	err = database.QueryRow("SELECT qname, qtype, protocol FROM dns_interactions").Scan(&qname, &qtype, &protocol)
	if err != nil {
		t.Fatalf("failed to query dns_interactions: %v", err)
	}
	if qname != "testtoken123.oastrix.local" {
		t.Errorf("expected qname testtoken123.oastrix.local, got %s", qname)
	}
	if qtype != int(dns.TypeA) {
		t.Errorf("expected qtype %d, got %d", dns.TypeA, qtype)
	}
	if protocol != "udp" {
		t.Errorf("expected protocol udp, got %s", protocol)
	}
}

func TestDNSServer_UnknownTokenDoesNotStore(t *testing.T) {
	tmpDB := t.TempDir() + "/test.db"
	database, err := db.Open(tmpDB)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer func() { _ = database.Close() }()
	defer func() { _ = os.Remove(tmpDB) }()

	srv := &DNSServer{
		Pipeline: setupPipeline(t, database),
		Domain:   "oastrix.local",
		PublicIP: "127.0.0.1",
		Logger:   zap.NewNop(),
	}

	req := new(dns.Msg)
	req.SetQuestion("unknowntoken.oastrix.local.", dns.TypeA)

	w := &mockResponseWriter{remoteAddr: &net.UDPAddr{IP: net.ParseIP("192.0.2.1"), Port: 12345}}
	srv.handleDNS(w, req)

	if w.msg == nil {
		t.Fatal("expected response message")
	}
	if len(w.msg.Answer) != 1 {
		t.Errorf("expected 1 answer (default response), got %d", len(w.msg.Answer))
	}

	var count int
	err = database.QueryRow("SELECT COUNT(*) FROM interactions").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count interactions: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 interactions for unknown token, got %d", count)
	}
}

func TestDNSServer_SOAQueryNotProcessedByPipeline(t *testing.T) {
	tmpDB := t.TempDir() + "/test.db"
	database, err := db.Open(tmpDB)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer func() { _ = database.Close() }()
	defer func() { _ = os.Remove(tmpDB) }()

	srv := &DNSServer{
		Pipeline: setupPipeline(t, database),
		Domain:   "oastrix.local",
		PublicIP: "127.0.0.1",
		Logger:   zap.NewNop(),
	}

	req := new(dns.Msg)
	req.SetQuestion("oastrix.local.", dns.TypeSOA)

	w := &mockResponseWriter{remoteAddr: &net.UDPAddr{IP: net.ParseIP("192.0.2.1"), Port: 12345}}
	srv.handleDNS(w, req)

	if w.msg == nil {
		t.Fatal("expected response message")
	}
	if len(w.msg.Answer) != 1 {
		t.Fatalf("expected 1 SOA answer, got %d", len(w.msg.Answer))
	}
	_, ok := w.msg.Answer[0].(*dns.SOA)
	if !ok {
		t.Errorf("expected SOA record, got %T", w.msg.Answer[0])
	}

	var count int
	err = database.QueryRow("SELECT COUNT(*) FROM interactions").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count interactions: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 interactions for SOA query, got %d", count)
	}
}

func TestDNSServer_NSQueryNotProcessedByPipeline(t *testing.T) {
	tmpDB := t.TempDir() + "/test.db"
	database, err := db.Open(tmpDB)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer func() { _ = database.Close() }()
	defer func() { _ = os.Remove(tmpDB) }()

	srv := &DNSServer{
		Pipeline: setupPipeline(t, database),
		Domain:   "oastrix.local",
		PublicIP: "127.0.0.1",
		Logger:   zap.NewNop(),
	}

	req := new(dns.Msg)
	req.SetQuestion("oastrix.local.", dns.TypeNS)

	w := &mockResponseWriter{remoteAddr: &net.UDPAddr{IP: net.ParseIP("192.0.2.1"), Port: 12345}}
	srv.handleDNS(w, req)

	if w.msg == nil {
		t.Fatal("expected response message")
	}
	if len(w.msg.Answer) != 1 {
		t.Fatalf("expected 1 NS answer, got %d", len(w.msg.Answer))
	}
	_, ok := w.msg.Answer[0].(*dns.NS)
	if !ok {
		t.Errorf("expected NS record, got %T", w.msg.Answer[0])
	}

	var count int
	err = database.QueryRow("SELECT COUNT(*) FROM interactions").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count interactions: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 interactions for NS query, got %d", count)
	}
}
