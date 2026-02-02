package events

import (
	"net/http"

	"github.com/miekg/dns"
)

// Event wraps an interaction draft with its assigned ID after storage.
type Event struct {
	Draft         *InteractionDraft
	InteractionID int64
}

// HTTPEvent extends Event with HTTP-specific request and response data.
type HTTPEvent struct {
	Event
	Req     *http.Request
	Resp    *HTTPResponsePlan
	Scratch map[string]any
}

// DNSEvent extends Event with DNS-specific request and response data.
type DNSEvent struct {
	Event
	Req      *dns.Msg
	Resp     *DNSResponsePlan
	QNameRaw string
}
