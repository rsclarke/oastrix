package acme

import (
	"strings"
	"sync"
)

type TXTStore struct {
	mu      sync.RWMutex
	records map[string]map[string]struct{}
}

func NewTXTStore() *TXTStore {
	return &TXTStore{
		records: make(map[string]map[string]struct{}),
	}
}

func (s *TXTStore) Add(fqdn, value string) {
	fqdn = NormalizeName(fqdn)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.records[fqdn] == nil {
		s.records[fqdn] = make(map[string]struct{})
	}
	s.records[fqdn][value] = struct{}{}
}

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

func NormalizeName(name string) string {
	name = strings.ToLower(name)
	name = strings.TrimSuffix(name, ".")
	return name
}
