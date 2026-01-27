package acme

import (
	"sort"
	"testing"
)

func TestTXTStore_AddGet(t *testing.T) {
	s := NewTXTStore()
	s.Add("example.com", "test-value")
	vals := s.Get("example.com")
	if len(vals) != 1 || vals[0] != "test-value" {
		t.Errorf("expected [test-value], got %v", vals)
	}
}

func TestTXTStore_MultipleValues(t *testing.T) {
	s := NewTXTStore()
	s.Add("example.com", "value1")
	s.Add("example.com", "value2")
	s.Add("example.com", "value3")
	vals := s.Get("example.com")
	if len(vals) != 3 {
		t.Errorf("expected 3 values, got %d", len(vals))
	}
	sort.Strings(vals)
	expected := []string{"value1", "value2", "value3"}
	for i, v := range expected {
		if vals[i] != v {
			t.Errorf("expected %s at index %d, got %s", v, i, vals[i])
		}
	}
}

func TestTXTStore_Remove(t *testing.T) {
	s := NewTXTStore()
	s.Add("example.com", "value1")
	s.Add("example.com", "value2")
	s.Remove("example.com", "value1")
	vals := s.Get("example.com")
	if len(vals) != 1 || vals[0] != "value2" {
		t.Errorf("expected [value2], got %v", vals)
	}
}

func TestTXTStore_RemoveNonexistent(t *testing.T) {
	s := NewTXTStore()
	s.Remove("nonexistent.com", "value")
	s.Add("example.com", "value")
	s.Remove("example.com", "other-value")
	vals := s.Get("example.com")
	if len(vals) != 1 {
		t.Errorf("expected 1 value, got %d", len(vals))
	}
}

func TestTXTStore_GetEmpty(t *testing.T) {
	s := NewTXTStore()
	vals := s.Get("nonexistent.com")
	if vals == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(vals) != 0 {
		t.Errorf("expected empty slice, got %v", vals)
	}
}

func TestTXTStore_Normalization(t *testing.T) {
	s := NewTXTStore()
	s.Add("Example.COM.", "value1")
	vals := s.Get("example.com")
	if len(vals) != 1 || vals[0] != "value1" {
		t.Errorf("expected [value1], got %v", vals)
	}
	s.Add("example.com", "value2")
	vals = s.Get("Example.COM.")
	if len(vals) != 2 {
		t.Errorf("expected 2 values, got %d", len(vals))
	}
	s.Remove("EXAMPLE.com.", "value1")
	vals = s.Get("example.com")
	if len(vals) != 1 || vals[0] != "value2" {
		t.Errorf("expected [value2], got %v", vals)
	}
}

func TestNormalizeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"example.com", "example.com"},
		{"Example.COM", "example.com"},
		{"example.com.", "example.com"},
		{"EXAMPLE.COM.", "example.com"},
		{"_acme-challenge.Example.COM.", "_acme-challenge.example.com"},
		{"", ""},
	}
	for _, tc := range tests {
		got := NormalizeName(tc.input)
		if got != tc.expected {
			t.Errorf("NormalizeName(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}
