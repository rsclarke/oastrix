// Package acme handles automatic TLS certificate management via ACME.
package acme

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/caddyserver/certmagic"
	certmagicsqlite "github.com/rsclarke/certmagic-sqlite"
	"go.uber.org/zap"
)

// Manager handles automatic certificate acquisition and renewal via ACME.
type Manager struct {
	Domain   string
	Email    string
	PublicIP string
	Staging  bool
	DB       *sql.DB
	TXTStore *TXTStore
	Logger   *zap.Logger

	dnsConfig *certmagic.Config
	ipConfig  *certmagic.Config
	storage   *certmagicsqlite.SQLiteStorage
}

// SetLogger configures the global certmagic loggers.
// Call this before starting any HTTP servers that handle ACME challenges.
func SetLogger(logger *zap.Logger) {
	if logger == nil {
		logger = zap.NewNop()
	}
	certmagic.Default.Logger = logger
	certmagic.DefaultACME.Logger = logger
}

// NewManager creates a new ACME manager.
func NewManager(domain, email string, db *sql.DB, staging bool, store *TXTStore, publicIP string, logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	// Set global certmagic loggers early, before any HTTP challenges arrive
	certmagic.Default.Logger = logger
	certmagic.DefaultACME.Logger = logger

	return &Manager{
		Domain:   domain,
		Email:    email,
		PublicIP: publicIP,
		Staging:  staging,
		DB:       db,
		TXTStore: store,
		Logger:   logger,
	}
}

// newBaseConfig creates a new certmagic config with common settings.
func (m *Manager) newBaseConfig() *certmagic.Config {
	certmagic.Default.Logger = m.Logger
	certmagic.DefaultACME.Logger = m.Logger
	cfg := certmagic.NewDefault()
	cfg.Storage = m.storage
	cfg.Logger = m.Logger
	return cfg
}

// Manage obtains and manages certificates for the domain and wildcard.
// This should be called after the DNS server is started.
func (m *Manager) Manage(ctx context.Context) error {
	// Create SQLite storage for certificates using the shared database
	hostname, _ := os.Hostname()
	storage, err := certmagicsqlite.NewWithDB(m.DB, certmagicsqlite.WithOwnerID(hostname))
	if err != nil {
		return fmt.Errorf("create certmagic storage: %w", err)
	}
	m.storage = storage

	// Create the DNS config using the base config helper
	m.dnsConfig = m.newBaseConfig()

	// Configure the ACME issuer
	var caURL string
	if m.Staging {
		caURL = certmagic.LetsEncryptStagingCA
	} else {
		caURL = certmagic.LetsEncryptProductionCA
	}

	// Create DNS provider using our TXTStore
	dnsProvider := &Provider{Store: m.TXTStore}

	issuer := certmagic.NewACMEIssuer(m.dnsConfig, certmagic.ACMEIssuer{
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

	m.dnsConfig.Issuers = []certmagic.Issuer{issuer}

	// Manage certificates sequentially with a delay between them.
	// Both domain and wildcard use the same _acme-challenge.<domain> TXT record.
	// Let's Encrypt's secondary validators may cache the first TXT record,
	// causing the wildcard challenge to fail if issued too quickly.
	// We issue them separately with a delay to allow caches to expire.

	// First: obtain certificate for the apex domain
	if err := m.dnsConfig.ManageSync(ctx, []string{m.Domain}); err != nil {
		return fmt.Errorf("manage certificate for %s: %w", m.Domain, err)
	}

	// Wait for DNS caches at validators to expire (TXT TTL is 1s, add buffer)
	time.Sleep(10 * time.Second)

	// Second: obtain certificate for the wildcard
	if err := m.dnsConfig.ManageSync(ctx, []string{"*." + m.Domain}); err != nil {
		return fmt.Errorf("manage certificate for *.%s: %w", m.Domain, err)
	}

	// Third: obtain certificate for the public IP via HTTP-01
	// Only IPv4 is supported; IPv6 HTTP-01 has upstream bugs.
	// See: https://github.com/caddyserver/caddy/issues/7399
	if m.PublicIP != "" {
		ip := net.ParseIP(m.PublicIP)
		if ip == nil {
			return fmt.Errorf("invalid public IP: %s", m.PublicIP)
		}

		// Skip IPv6 - HTTP-01 for IPv6 has upstream bugs
		if ip.To4() == nil {
			m.Logger.Warn("skipping IP certificate for IPv6 address (not yet supported)", zap.String("ip", m.PublicIP))
		} else {
			m.ipConfig = m.newBaseConfig()

			ipIssuer := certmagic.NewACMEIssuer(m.ipConfig, certmagic.ACMEIssuer{
				CA:                      caURL,
				Email:                   m.Email,
				Agreed:                  true,
				Profile:                 "shortlived",
				DisableTLSALPNChallenge: true, // Use HTTP-01 only
				Logger:                  m.Logger,
			})
			m.ipConfig.Issuers = []certmagic.Issuer{ipIssuer}

			m.Logger.Info("obtaining IP certificate via HTTP-01", zap.String("ip", m.PublicIP))
			if err := m.ipConfig.ManageSync(ctx, []string{m.PublicIP}); err != nil {
				m.Logger.Warn("failed to obtain IP certificate", zap.String("ip", m.PublicIP), zap.Error(err))
				// Don't fail startup - IP cert is optional, HTTP interactions still work
			} else {
				m.Logger.Info("IP certificate obtained", zap.String("ip", m.PublicIP))
			}
		}
	}

	return nil
}

// TLSConfig returns a TLS configuration that uses the managed certificates.
func (m *Manager) TLSConfig() *tls.Config {
	if m.dnsConfig == nil {
		return nil
	}

	// If no IP config, just use DNS config
	if m.ipConfig == nil {
		return m.dnsConfig.TLSConfig()
	}

	// Compose TLS config that routes to correct cert based on SNI
	dnsTLS := m.dnsConfig.TLSConfig()
	ipTLS := m.ipConfig.TLSConfig()

	return &tls.Config{
		GetCertificate: func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
			sni := strings.Trim(chi.ServerName, "[]")

			// Empty SNI or matching IP → prefer IP cert
			if sni == "" || sni == m.PublicIP {
				if cert, err := ipTLS.GetCertificate(chi); err == nil && cert != nil {
					return cert, nil
				}
			}
			return dnsTLS.GetCertificate(chi)
		},
		NextProtos: []string{"h2", "http/1.1", "acme-tls/1"},
	}
}
