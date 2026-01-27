package models

type APIKey struct {
	ID        int64
	KeyPrefix string
	KeyHash   []byte
	CreatedAt int64
	RevokedAt *int64
}

type Token struct {
	ID        int64
	Token     string
	APIKeyID  *int64
	CreatedAt int64
	Label     *string
}

type Interaction struct {
	ID         int64
	TokenID    int64
	Kind       string
	OccurredAt int64
	RemoteIP   string
	RemotePort int
	TLS        bool
	Summary    string
}

type HTTPInteraction struct {
	InteractionID  int64
	Method         string
	Scheme         string
	Host           string
	Path           string
	Query          string
	HTTPVersion    string
	RequestHeaders string
	RequestBody    []byte
}

type DNSInteraction struct {
	InteractionID int64
	QName         string
	QType         int
	QClass        int
	RD            int
	Opcode        int
	DNSID         int
	Protocol      string
}
