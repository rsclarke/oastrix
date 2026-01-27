package acme

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/caddyserver/certmagic"
	"go.uber.org/zap"
)

// Manager handles automatic certificate acquisition and renewal via ACME
type Manager struct {
	Domain     string
	Email      string
	Staging    bool
	StorageDir string
	TXTStore   *TXTStore
	Logger     *zap.Logger

	config *certmagic.Config
}

// NewManager creates a new ACME manager
func NewManager(domain, email, storageDir string, staging bool, store *TXTStore, logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{
		Domain:     domain,
		Email:      email,
		Staging:    staging,
		StorageDir: storageDir,
		TXTStore:   store,
		Logger:     logger,
	}
}

// Manage obtains and manages certificates for the domain and wildcard
// This should be called after the DNS server is started
func (m *Manager) Manage(ctx context.Context) error {
	// Create file storage for certificates
	storage := &certmagic.FileStorage{Path: m.StorageDir}

	// Set the global default logger BEFORE calling NewDefault() so that
	// internal certmagic components (cache maintainer, ACME client, etc.)
	// inherit our zap logger when the cache is created
	certmagic.Default.Logger = m.Logger

	// Create the config
	m.config = certmagic.NewDefault()
	m.config.Storage = storage
	m.config.Logger = m.Logger

	// Configure the ACME issuer
	var caURL string
	if m.Staging {
		caURL = certmagic.LetsEncryptStagingCA
	} else {
		caURL = certmagic.LetsEncryptProductionCA
	}

	// Create DNS provider using our TXTStore
	dnsProvider := &Provider{Store: m.TXTStore}

	issuer := certmagic.NewACMEIssuer(m.config, certmagic.ACMEIssuer{
		CA:     caURL,
		Email:  m.Email,
		Agreed: true,
		Logger: m.Logger,
		DNS01Solver: &certmagic.DNS01Solver{
			DNSManager: certmagic.DNSManager{
				DNSProvider: dnsProvider,
				Logger:      m.Logger,
			},
		},
	})

	m.config.Issuers = []certmagic.Issuer{issuer}

	// Manage certificates sequentially with a delay between them.
	// Both domain and wildcard use the same _acme-challenge.<domain> TXT record.
	// Let's Encrypt's secondary validators may cache the first TXT record,
	// causing the wildcard challenge to fail if issued too quickly.
	// We issue them separately with a delay to allow caches to expire.

	// First: obtain certificate for the apex domain
	if err := m.config.ManageSync(ctx, []string{m.Domain}); err != nil {
		return fmt.Errorf("manage certificate for %s: %w", m.Domain, err)
	}

	// Wait for DNS caches at validators to expire (TXT TTL is 1s, add buffer)
	time.Sleep(10 * time.Second)

	// Second: obtain certificate for the wildcard
	if err := m.config.ManageSync(ctx, []string{"*." + m.Domain}); err != nil {
		return fmt.Errorf("manage certificate for *.%s: %w", m.Domain, err)
	}

	return nil
}

// TLSConfig returns a TLS configuration that uses the managed certificates
func (m *Manager) TLSConfig() *tls.Config {
	if m.config == nil {
		return nil
	}
	return m.config.TLSConfig()
}
