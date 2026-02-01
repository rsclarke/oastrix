package acme

import (
	"context"
	"net/netip"
	"testing"

	"github.com/libdns/libdns"
)

func TestProvider_AppendRecords(t *testing.T) {
	store := NewTXTStore()
	p := &Provider{Store: store}

	recs := []libdns.Record{
		libdns.TXT{Name: "_acme-challenge", Text: "test-token-123"},
	}

	_, err := p.AppendRecords(context.Background(), "example.com.", recs)
	if err != nil {
		t.Fatalf("AppendRecords failed: %v", err)
	}

	vals := store.Get("_acme-challenge.example.com")
	if len(vals) != 1 || vals[0] != "test-token-123" {
		t.Errorf("expected [test-token-123], got %v", vals)
	}
}

func TestProvider_DeleteRecords(t *testing.T) {
	store := NewTXTStore()
	store.Add("_acme-challenge.example.com", "test-token-123")

	p := &Provider{Store: store}

	recs := []libdns.Record{
		libdns.TXT{Name: "_acme-challenge", Text: "test-token-123"},
	}

	_, err := p.DeleteRecords(context.Background(), "example.com.", recs)
	if err != nil {
		t.Fatalf("DeleteRecords failed: %v", err)
	}

	vals := store.Get("_acme-challenge.example.com")
	if len(vals) != 0 {
		t.Errorf("expected empty, got %v", vals)
	}
}

func TestProvider_IgnoresNonTXT(t *testing.T) {
	store := NewTXTStore()
	p := &Provider{Store: store}

	recs := []libdns.Record{
		libdns.Address{Name: "www", IP: netip.MustParseAddr("192.168.1.1")},
	}

	_, err := p.AppendRecords(context.Background(), "example.com.", recs)
	if err != nil {
		t.Fatalf("AppendRecords failed: %v", err)
	}

	if vals := store.Get("www.example.com"); len(vals) != 0 {
		t.Errorf("expected empty store, got %v", vals)
	}
}

func TestProvider_MultipleValuesForSameName(t *testing.T) {
	store := NewTXTStore()
	p := &Provider{Store: store}
	ctx := context.Background()
	zone := "example.com."

	apexRecs := []libdns.Record{
		libdns.TXT{Name: "_acme-challenge", Text: "token-apex"},
	}
	wildcardRecs := []libdns.Record{
		libdns.TXT{Name: "_acme-challenge", Text: "token-wildcard"},
	}

	if _, err := p.AppendRecords(ctx, zone, apexRecs); err != nil {
		t.Fatalf("AppendRecords (apex) failed: %v", err)
	}
	if _, err := p.AppendRecords(ctx, zone, wildcardRecs); err != nil {
		t.Fatalf("AppendRecords (wildcard) failed: %v", err)
	}

	vals := store.Get("_acme-challenge.example.com")
	if len(vals) != 2 {
		t.Fatalf("expected 2 values, got %d: %v", len(vals), vals)
	}

	if _, err := p.DeleteRecords(ctx, zone, apexRecs); err != nil {
		t.Fatalf("DeleteRecords (apex) failed: %v", err)
	}

	vals = store.Get("_acme-challenge.example.com")
	if len(vals) != 1 {
		t.Fatalf("expected 1 value after delete, got %d: %v", len(vals), vals)
	}
	if vals[0] != "token-wildcard" {
		t.Errorf("expected token-wildcard to remain, got %s", vals[0])
	}

	if _, err := p.DeleteRecords(ctx, zone, wildcardRecs); err != nil {
		t.Fatalf("DeleteRecords (wildcard) failed: %v", err)
	}

	vals = store.Get("_acme-challenge.example.com")
	if len(vals) != 0 {
		t.Errorf("expected empty after all deletes, got %v", vals)
	}
}

func TestAbsoluteName(t *testing.T) {
	tests := []struct {
		zone     string
		name     string
		expected string
	}{
		{"example.com.", "_acme-challenge", "_acme-challenge.example.com"},
		{"example.com", "_acme-challenge", "_acme-challenge.example.com"},
		{"example.com.", "_acme-challenge.example.com.", "_acme-challenge.example.com"},
		{"EXAMPLE.COM.", "_ACME-CHALLENGE", "_acme-challenge.example.com"},
		{"example.com.", "", "example.com"},
		{"example.com.", ".", "example.com"},
		{"example.com", "sub.example.com", "sub.example.com"},
	}

	for _, tc := range tests {
		got := absoluteName(tc.zone, tc.name)
		if got != tc.expected {
			t.Errorf("absoluteName(%q, %q) = %q, want %q", tc.zone, tc.name, got, tc.expected)
		}
	}
}
