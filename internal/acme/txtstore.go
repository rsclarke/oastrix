package acme

import (
	"strings"
	"sync"
)

// TXTStore is a thread-safe store for DNS TXT records used in ACME challenges.
type TXTStore struct {
	mu      sync.RWMutex
	records map[string]map[string]struct{}
}

// NewTXTStore creates a new empty TXT record store.
func NewTXTStore() *TXTStore {
	return &TXTStore{
		records: make(map[string]map[string]struct{}),
	}
}

// Add inserts a TXT record value for the given FQDN.
func (s *TXTStore) Add(fqdn, value string) {
	fqdn = NormalizeName(fqdn)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.records[fqdn] == nil {
		s.records[fqdn] = make(map[string]struct{})
	}
	s.records[fqdn][value] = struct{}{}
}

// Remove deletes a TXT record value for the given FQDN.
func (s *TXTStore) Remove(fqdn, value string) {
	fqdn = NormalizeName(fqdn)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.records[fqdn] == nil {
		return
	}
	delete(s.records[fqdn], value)
	if len(s.records[fqdn]) == 0 {
		delete(s.records, fqdn)
	}
}

// Get returns all TXT record values for the given FQDN.
func (s *TXTStore) Get(fqdn string) []string {
	fqdn = NormalizeName(fqdn)
	s.mu.RLock()
	defer s.mu.RUnlock()
	vals := s.records[fqdn]
	if vals == nil {
		return []string{}
	}
	result := make([]string, 0, len(vals))
	for v := range vals {
		result = append(result, v)
	}
	return result
}

// NormalizeName lowercases and removes trailing dots from a DNS name.
func NormalizeName(name string) string {
	name = strings.ToLower(name)
	name = strings.TrimSuffix(name, ".")
	return name
}
