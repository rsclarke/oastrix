package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rsclarke/oastrix/internal/acme"
	"github.com/rsclarke/oastrix/internal/auth"
	"github.com/rsclarke/oastrix/internal/db"
	"github.com/rsclarke/oastrix/internal/logging"
	"github.com/rsclarke/oastrix/internal/server"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var serverFlags struct {
	httpPort    int
	httpsPort   int
	apiPort     int
	dnsPort     int
	tlsCert     string
	tlsKey      string
	domain      string
	dbPath      string
	pepper      string
	noACME      bool
	acmeEmail   string
	acmeStaging bool
	publicIP    string
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start all listeners (HTTP, HTTPS, DNS, API)",
	Long: `Start the oastrix server with HTTP, HTTPS, DNS, and API listeners.

TLS Modes:
  By default, ACME is enabled and certificates are automatically obtained
  from Let's Encrypt using DNS-01 challenges. The DNS server must be
  publicly reachable on port 53 for ACME to work.

  --tls-cert + --tls-key  → Manual TLS mode (use provided certificates)
  --no-acme               → HTTP only (no HTTPS server)
  (neither)               → ACME mode (automatic Let's Encrypt certificates)

Notes:
  Ports 80, 443, and 53 require root or 'setcap cap_net_bind_service'.
  Certificates are stored in <db-dir>/certmagic/.`,
	RunE: runServer,
}

func init() {
	rootCmd.AddCommand(serverCmd)

	serverCmd.Flags().IntVar(&serverFlags.httpPort, "http-port", getEnvInt("OASTRIX_HTTP_PORT", 80), "HTTP port to listen on")
	serverCmd.Flags().IntVar(&serverFlags.httpsPort, "https-port", getEnvInt("OASTRIX_HTTPS_PORT", 443), "HTTPS port to listen on")
	serverCmd.Flags().IntVar(&serverFlags.apiPort, "api-port", getEnvInt("OASTRIX_API_PORT", 8081), "API port to listen on")
	serverCmd.Flags().IntVar(&serverFlags.dnsPort, "dns-port", getEnvInt("OASTRIX_DNS_PORT", 53), "DNS port to listen on (53 requires root)")
	serverCmd.Flags().StringVar(&serverFlags.tlsCert, "tls-cert", "", "path to TLS certificate file (enables manual TLS mode)")
	serverCmd.Flags().StringVar(&serverFlags.tlsKey, "tls-key", "", "path to TLS key file (enables manual TLS mode)")
	serverCmd.Flags().StringVar(&serverFlags.domain, "domain", getEnv("OASTRIX_DOMAIN", "localhost"), "domain for token extraction")
	serverCmd.Flags().StringVar(&serverFlags.publicIP, "public-ip", getEnv("OASTRIX_PUBLIC_IP", ""), "public IP for DNS responses (required for ACME)")
	serverCmd.Flags().StringVar(&serverFlags.dbPath, "db", getEnv("OASTRIX_DB", "oastrix.db"), "database path")
	serverCmd.Flags().StringVar(&serverFlags.pepper, "pepper", os.Getenv("OASTRIX_PEPPER"), "HMAC pepper for API key hashing")
	serverCmd.Flags().BoolVar(&serverFlags.noACME, "no-acme", false, "disable automatic TLS (ACME)")
	serverCmd.Flags().StringVar(&serverFlags.acmeEmail, "acme-email", "", "email for Let's Encrypt notifications")
	serverCmd.Flags().BoolVar(&serverFlags.acmeStaging, "acme-staging", false, "use Let's Encrypt staging CA")
}

func runServer(cmd *cobra.Command, args []string) error {
	pepper := serverFlags.pepper
	if pepper == "" {
		pepperBytes := make([]byte, 32)
		if _, err := rand.Read(pepperBytes); err != nil {
			return fmt.Errorf("generate pepper: %w", err)
		}
		pepper = base64.StdEncoding.EncodeToString(pepperBytes)
	}
	pepperBytes := []byte(pepper)

	database, err := db.Open(serverFlags.dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	count, err := db.CountAPIKeys(database)
	if err != nil {
		return fmt.Errorf("count API keys: %w", err)
	}
	if count == 0 {
		displayKey, prefix, hash, err := auth.GenerateAPIKey(pepperBytes)
		if err != nil {
			return fmt.Errorf("generate API key: %w", err)
		}
		_, err = db.CreateAPIKey(database, prefix, hash)
		if err != nil {
			return fmt.Errorf("create API key: %w", err)
		}
		fmt.Println("=============================================================")
		fmt.Println("API KEY CREATED (save this, it will not be shown again):")
		fmt.Println(displayKey)
		fmt.Println("=============================================================")
	}

	manualTLS := serverFlags.tlsCert != "" && serverFlags.tlsKey != ""
	acmeMode := !manualTLS && !serverFlags.noACME

	if acmeMode && serverFlags.publicIP == "" {
		return fmt.Errorf("--public-ip is required for ACME mode (or use --no-acme)")
	}

	var txtStore *acme.TXTStore
	if acmeMode {
		txtStore = acme.NewTXTStore()
		acme.SetLogger(logger.Named("certmagic"))
	}

	httpSrv := &server.HTTPServer{
		DB:       database,
		Domain:   serverFlags.domain,
		PublicIP: serverFlags.publicIP,
		Logger:   logger.Named("http"),
	}

	httpErrLog, _ := zap.NewStdLogAt(logger.Named("http"), zapcore.ErrorLevel)
	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", serverFlags.httpPort),
		Handler:           httpSrv,
		ErrorLog:          httpErrLog,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
	}

	go func() {
		logger.Info("starting http server", logging.Port(serverFlags.httpPort))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server error", zap.Error(err))
		}
	}()

	apiSrv := &server.APIServer{
		DB:       database,
		Domain:   serverFlags.domain,
		PublicIP: serverFlags.publicIP,
		Pepper:   pepperBytes,
		Logger:   logger.Named("api"),
	}

	apiErrLog, _ := zap.NewStdLogAt(logger.Named("api"), zapcore.ErrorLevel)
	apiServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", serverFlags.apiPort),
		Handler:           apiSrv.Handler(),
		ErrorLog:          apiErrLog,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
	}

	go func() {
		logger.Info("starting api server", logging.Port(serverFlags.apiPort))
		if err := apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("api server error", zap.Error(err))
		}
	}()

	dnsSrv := &server.DNSServer{
		DB:       database,
		Domain:   serverFlags.domain,
		PublicIP: serverFlags.publicIP,
		TXTStore: txtStore,
		Logger:   logger.Named("dns"),
	}
	if err := dnsSrv.Start(serverFlags.dnsPort, serverFlags.dnsPort); err != nil {
		return fmt.Errorf("start DNS server: %w", err)
	}

	var httpsServer *http.Server
	httpsErrLog, _ := zap.NewStdLogAt(logger.Named("https"), zapcore.ErrorLevel)
	if acmeMode {
		manager := acme.NewManager(serverFlags.domain, serverFlags.acmeEmail, database, serverFlags.acmeStaging, txtStore, serverFlags.publicIP, logger.Named("certmagic"))

		logger.Info("starting acme certificate acquisition", logging.Domain(serverFlags.domain), zap.Bool("staging", serverFlags.acmeStaging))
		ctx := context.Background()
		if err := manager.Manage(ctx); err != nil {
			return fmt.Errorf("ACME certificate acquisition: %w", err)
		}
		logger.Info("acme certificate obtained", logging.Domain(serverFlags.domain))

		httpsServer = &http.Server{
			Addr:              fmt.Sprintf(":%d", serverFlags.httpsPort),
			Handler:           httpSrv,
			TLSConfig:         manager.TLSConfig(),
			ErrorLog:          httpsErrLog,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
		}

		go func() {
			logger.Info("starting https server", logging.Port(serverFlags.httpsPort), logging.TLSMode("acme"))
			if err := httpsServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				logger.Error("https server error", zap.Error(err))
			}
		}()
	} else if manualTLS {
		cert, err := tls.LoadX509KeyPair(serverFlags.tlsCert, serverFlags.tlsKey)
		if err != nil {
			return fmt.Errorf("load TLS certificate: %w", err)
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
		}

		httpsServer = &http.Server{
			Addr:              fmt.Sprintf(":%d", serverFlags.httpsPort),
			Handler:           httpSrv,
			TLSConfig:         tlsConfig,
			ErrorLog:          httpsErrLog,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
		}

		go func() {
			logger.Info("starting https server", logging.Port(serverFlags.httpsPort), logging.TLSMode("manual"))
			if err := httpsServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				logger.Error("https server error", zap.Error(err))
			}
		}()
	} else {
		logger.Info("https disabled", zap.String("reason", "no-acme specified without manual TLS certificates"))
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if httpsServer != nil {
		httpsServer.Shutdown(ctx)
	}
	httpServer.Shutdown(ctx)
	apiServer.Shutdown(ctx)
	dnsSrv.Shutdown()

	return nil
}
