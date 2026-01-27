package server

import (
	"testing"
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
