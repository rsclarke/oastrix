// Package types defines the API request and response types.
package types

// CreateTokenRequest is the request body for creating a new token.
type CreateTokenRequest struct {
	Label string `json:"label,omitempty"`
}

// CreateTokenResponse is the response body for token creation.
type CreateTokenResponse struct {
	Token    string            `json:"token"`
	Payloads map[string]string `json:"payloads"`
}

// TokenInfo represents a token with its metadata.
type TokenInfo struct {
	Token            string  `json:"token"`
	Label            *string `json:"label"`
	CreatedAt        string  `json:"created_at"`
	InteractionCount int     `json:"interaction_count"`
}

// ListTokensResponse is the response body for listing tokens.
type ListTokensResponse struct {
	Tokens []TokenInfo `json:"tokens"`
}

// InteractionResponse represents a single recorded interaction.
type InteractionResponse struct {
	ID         int64                  `json:"id"`
	Kind       string                 `json:"kind"`
	OccurredAt string                 `json:"occurred_at"`
	RemoteIP   string                 `json:"remote_ip"`
	RemotePort int                    `json:"remote_port"`
	TLS        bool                   `json:"tls"`
	Summary    string                 `json:"summary"`
	HTTP       *HTTPInteractionDetail `json:"http,omitempty"`
	DNS        *DNSInteractionDetail  `json:"dns,omitempty"`
}

// HTTPInteractionDetail contains HTTP-specific interaction details.
type HTTPInteractionDetail struct {
	Method  string              `json:"method"`
	Scheme  string              `json:"scheme"`
	Host    string              `json:"host"`
	Path    string              `json:"path"`
	Query   string              `json:"query"`
	Headers map[string][]string `json:"headers"`
	Body    string              `json:"body"`
}

// DNSInteractionDetail contains DNS-specific interaction details.
type DNSInteractionDetail struct {
	QName    string `json:"qname"`
	QType    int    `json:"qtype"`
	QClass   int    `json:"qclass"`
	RD       bool   `json:"rd"`
	Opcode   int    `json:"opcode"`
	DNSID    int    `json:"dns_id"`
	Protocol string `json:"protocol"`
}

// GetInteractionsResponse is the response body for retrieving interactions.
type GetInteractionsResponse struct {
	Token        string                `json:"token"`
	Interactions []InteractionResponse `json:"interactions"`
}

// DeleteTokenResponse is the response body for token deletion.
type DeleteTokenResponse struct {
	Deleted bool `json:"deleted"`
}

// ErrorResponse represents an API error response.
type ErrorResponse struct {
	Error string `json:"error"`
}
