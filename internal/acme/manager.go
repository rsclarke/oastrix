package acme

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/caddyserver/certmagic"
	certmagicsqlite "github.com/rsclarke/certmagic-sqlite"
	"go.uber.org/zap"
)

// Manager handles automatic certificate acquisition and renewal via ACME
type Manager struct {
	Domain   string
	Email    string
	Staging  bool
	DB       *sql.DB
	TXTStore *TXTStore
	Logger   *zap.Logger

	config  *certmagic.Config
	storage *certmagicsqlite.SQLiteStorage
}

// NewManager creates a new ACME manager
func NewManager(domain, email string, db *sql.DB, staging bool, store *TXTStore, logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{
		Domain:   domain,
		Email:    email,
		Staging:  staging,
		DB:       db,
		TXTStore: store,
		Logger:   logger,
	}
}

// Manage obtains and manages certificates for the domain and wildcard
// This should be called after the DNS server is started
func (m *Manager) Manage(ctx context.Context) error {
	// Create SQLite storage for certificates using the shared database
	hostname, _ := os.Hostname()
	storage, err := certmagicsqlite.NewWithDB(m.DB, certmagicsqlite.WithOwnerID(hostname))
	if err != nil {
		return fmt.Errorf("create certmagic storage: %w", err)
	}
	m.storage = storage

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
