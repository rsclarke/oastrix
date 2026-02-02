// Package events defines the core types used throughout the plugin framework.
package events

import "github.com/miekg/dns"

// Kind represents the type of interaction (HTTP or DNS).
type Kind string

// Interaction kinds.
const (
	KindHTTP Kind = "http"
	KindDNS  Kind = "dns"
)

// InteractionDraft represents an interaction in progress before storage.
type InteractionDraft struct {
	TokenValue string
	TokenID    int64
	Kind       Kind
	OccurredAt int64
	RemoteIP   string
	RemotePort int
	TLS        bool
	Summary    string
	HTTP       *HTTPDraft
	DNS        *DNSDraft
	Attributes map[string]any
	Drop       bool
}

// HTTPDraft contains HTTP-specific interaction details.
type HTTPDraft struct {
	Method, Scheme, Host, Path, Query, Proto string
	Headers                                  map[string][]string
	Body                                     []byte
}

// DNSDraft contains DNS-specific interaction details.
type DNSDraft struct {
	QName    string
	QType    int
	QClass   int
	RD       int
	Opcode   int
	DNSID    int
	Protocol string
}

// HTTPResponsePlan describes the HTTP response to be sent.
type HTTPResponsePlan struct {
	Status  int
	Headers map[string]string
	Body    []byte
	Handled bool
}

// DNSResponsePlan describes the DNS response to be sent.
type DNSResponsePlan struct {
	RCode   int
	Answers []dns.RR
	Handled bool
}
