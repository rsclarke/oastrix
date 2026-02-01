package db

import (
	"database/sql"
	"time"

	"github.com/rsclarke/oastrix/internal/models"
)

// CreateInteraction inserts a new interaction record and returns its ID.
func CreateInteraction(d *sql.DB, tokenID int64, kind string, remoteIP string, remotePort int, tls bool, summary string) (int64, error) {
	tlsVal := 0
	if tls {
		tlsVal = 1
	}
	result, err := d.Exec(
		"INSERT INTO interactions (token_id, kind, occurred_at, remote_ip, remote_port, tls, summary) VALUES (?, ?, ?, ?, ?, ?, ?)",
		tokenID, kind, time.Now().Unix(), remoteIP, remotePort, tlsVal, summary,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// CreateHTTPInteraction inserts HTTP-specific details for an interaction.
func CreateHTTPInteraction(d *sql.DB, interactionID int64, method, scheme, host, path, query, httpVersion string, headers string, body []byte) error {
	_, err := d.Exec(
		"INSERT INTO http_interactions (interaction_id, method, scheme, host, path, query, http_version, request_headers, request_body) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		interactionID, method, scheme, host, path, query, httpVersion, headers, body,
	)
	return err
}

// GetInteractionsByToken retrieves all interactions for a given token ID.
func GetInteractionsByToken(d *sql.DB, tokenID int64) ([]models.Interaction, error) {
	rows, err := d.Query(
		"SELECT id, token_id, kind, occurred_at, remote_ip, remote_port, tls, summary FROM interactions WHERE token_id = ? ORDER BY occurred_at DESC",
		tokenID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var interactions []models.Interaction
	for rows.Next() {
		var i models.Interaction
		var tlsVal int
		err := rows.Scan(&i.ID, &i.TokenID, &i.Kind, &i.OccurredAt, &i.RemoteIP, &i.RemotePort, &tlsVal, &i.Summary)
		if err != nil {
			return nil, err
		}
		i.TLS = tlsVal != 0
		interactions = append(interactions, i)
	}
	return interactions, rows.Err()
}

// GetHTTPInteraction retrieves HTTP-specific details for an interaction.
func GetHTTPInteraction(d *sql.DB, interactionID int64) (*models.HTTPInteraction, error) {
	row := d.QueryRow(
		"SELECT interaction_id, method, scheme, host, path, query, http_version, request_headers, request_body FROM http_interactions WHERE interaction_id = ?",
		interactionID,
	)
	var h models.HTTPInteraction
	err := row.Scan(&h.InteractionID, &h.Method, &h.Scheme, &h.Host, &h.Path, &h.Query, &h.HTTPVersion, &h.RequestHeaders, &h.RequestBody)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &h, nil
}

// CreateDNSInteraction inserts DNS-specific details for an interaction.
func CreateDNSInteraction(d *sql.DB, interactionID int64, qname string, qtype, qclass, rd, opcode, dnsID int, protocol string) error {
	_, err := d.Exec(
		"INSERT INTO dns_interactions (interaction_id, qname, qtype, qclass, rd, opcode, dns_id, protocol) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		interactionID, qname, qtype, qclass, rd, opcode, dnsID, protocol,
	)
	return err
}

// GetDNSInteraction retrieves DNS-specific details for an interaction.
func GetDNSInteraction(d *sql.DB, interactionID int64) (*models.DNSInteraction, error) {
	row := d.QueryRow(
		"SELECT interaction_id, qname, qtype, qclass, rd, opcode, dns_id, protocol FROM dns_interactions WHERE interaction_id = ?",
		interactionID,
	)
	var dns models.DNSInteraction
	err := row.Scan(&dns.InteractionID, &dns.QName, &dns.QType, &dns.QClass, &dns.RD, &dns.Opcode, &dns.DNSID, &dns.Protocol)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &dns, nil
}
