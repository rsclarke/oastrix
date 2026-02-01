// Package models defines the database entity types.
package models

// APIKey represents an API key record in the database.
type APIKey struct {
	ID        int64
	KeyPrefix string
	KeyHash   []byte
	CreatedAt int64
	RevokedAt *int64
}

// Token represents an OAST token record in the database.
type Token struct {
	ID        int64
	Token     string
	APIKeyID  *int64
	CreatedAt int64
	Label     *string
}

// Interaction represents a recorded interaction event.
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

// HTTPInteraction contains HTTP-specific details for an interaction.
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

// DNSInteraction contains DNS-specific details for an interaction.
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
