package api

type CreateTokenRequest struct {
	Label string `json:"label,omitempty"`
}

type CreateTokenResponse struct {
	Token    string            `json:"token"`
	Payloads map[string]string `json:"payloads"`
}

type TokenInfo struct {
	Token            string  `json:"token"`
	Label            *string `json:"label"`
	CreatedAt        string  `json:"created_at"`
	InteractionCount int     `json:"interaction_count"`
}

type ListTokensResponse struct {
	Tokens []TokenInfo `json:"tokens"`
}

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

type HTTPInteractionDetail struct {
	Method  string              `json:"method"`
	Scheme  string              `json:"scheme"`
	Host    string              `json:"host"`
	Path    string              `json:"path"`
	Query   string              `json:"query"`
	Headers map[string][]string `json:"headers"`
	Body    string              `json:"body"`
}

type DNSInteractionDetail struct {
	QName    string `json:"qname"`
	QType    int    `json:"qtype"`
	QClass   int    `json:"qclass"`
	RD       bool   `json:"rd"`
	Opcode   int    `json:"opcode"`
	DNSID    int    `json:"dns_id"`
	Protocol string `json:"protocol"`
}

type GetInteractionsResponse struct {
	Token        string                `json:"token"`
	Interactions []InteractionResponse `json:"interactions"`
}

type DeleteTokenResponse struct {
	Deleted bool `json:"deleted"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
