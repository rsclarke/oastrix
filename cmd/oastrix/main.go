package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rsclarke/oastrix/internal/acme"
	"github.com/rsclarke/oastrix/internal/auth"
	"github.com/rsclarke/oastrix/internal/client"
	"github.com/rsclarke/oastrix/internal/config"
	"github.com/rsclarke/oastrix/internal/db"
	"github.com/rsclarke/oastrix/internal/logging"
	"github.com/rsclarke/oastrix/internal/server"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	logger, err := logging.New(logging.FromEnv())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error initializing logger: %v\n", err)
		os.Exit(1)
	}
	defer logging.Sync(logger)

	switch os.Args[1] {
	case "server":
		if err := runServer(os.Args[2:], logger); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "generate":
		if err := runGenerate(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "interactions":
		if err := runInteractions(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "delete":
		if err := runDelete(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "list":
		if err := runList(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "--help", "-h", "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func runServer(args []string, logger *zap.Logger) error {
	serverCmd := flag.NewFlagSet("server", flag.ExitOnError)

	var httpPort int
	var httpsPort int
	var apiPort int
	var dnsPort int
	var tlsCert string
	var tlsKey string
	var domain string
	var dbPath string
	var pepper string
	var noACME bool
	var acmeEmail string
	var acmeStaging bool
	var publicIP string

	serverCmd.IntVar(&httpPort, "http-port", getEnvInt("OASTRIX_HTTP_PORT", 80), "HTTP port to listen on")
	serverCmd.IntVar(&httpsPort, "https-port", getEnvInt("OASTRIX_HTTPS_PORT", 443), "HTTPS port to listen on")
	serverCmd.IntVar(&apiPort, "api-port", getEnvInt("OASTRIX_API_PORT", 8081), "API port to listen on")
	serverCmd.IntVar(&dnsPort, "dns-port", getEnvInt("OASTRIX_DNS_PORT", 53), "DNS port to listen on (53 requires root)")
	serverCmd.StringVar(&tlsCert, "tls-cert", "", "path to TLS certificate file (enables manual TLS mode)")
	serverCmd.StringVar(&tlsKey, "tls-key", "", "path to TLS key file (enables manual TLS mode)")
	serverCmd.StringVar(&domain, "domain", getEnv("OASTRIX_DOMAIN", "localhost"), "domain for token extraction")
	serverCmd.StringVar(&publicIP, "public-ip", getEnv("OASTRIX_PUBLIC_IP", ""), "public IP for DNS responses (required for ACME)")
	serverCmd.StringVar(&dbPath, "db", getEnv("OASTRIX_DB", "oastrix.db"), "database path")
	serverCmd.StringVar(&pepper, "pepper", os.Getenv("OASTRIX_PEPPER"), "HMAC pepper for API key hashing")
	serverCmd.BoolVar(&noACME, "no-acme", false, "disable automatic TLS (ACME)")
	serverCmd.StringVar(&acmeEmail, "acme-email", "", "email for Let's Encrypt notifications")
	serverCmd.BoolVar(&acmeStaging, "acme-staging", false, "use Let's Encrypt staging CA")

	if err := serverCmd.Parse(args); err != nil {
		return err
	}

	if pepper == "" {
		loadedCfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		pepper = loadedCfg.Pepper
	}
	pepperBytes := []byte(pepper)

	database, err := db.Open(dbPath)
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

	// Determine TLS mode:
	// 1. Manual TLS: --tls-cert AND --tls-key provided
	// 2. No HTTPS: --no-acme is true (and no manual certs)
	// 3. ACME mode: default (automatic TLS via Let's Encrypt)
	manualTLS := tlsCert != "" && tlsKey != ""
	acmeMode := !manualTLS && !noACME

	if acmeMode && publicIP == "" {
		return fmt.Errorf("--public-ip is required for ACME mode (or use --no-acme)")
	}

	// Create TXTStore for ACME (needed for DNS server regardless of ACME mode)
	var txtStore *acme.TXTStore
	if acmeMode {
		txtStore = acme.NewTXTStore()
		// Set certmagic loggers before HTTP server starts to handle challenges
		acme.SetLogger(logger.Named("certmagic"))
	}

	httpSrv := &server.HTTPServer{
		DB:       database,
		Domain:   domain,
		PublicIP: publicIP,
		Logger:   logger.Named("http"),
	}

	httpErrLog, _ := zap.NewStdLogAt(logger.Named("http"), zapcore.ErrorLevel)
	httpServer := &http.Server{
		Addr:     fmt.Sprintf(":%d", httpPort),
		Handler:  httpSrv,
		ErrorLog: httpErrLog,
	}

	go func() {
		logger.Info("starting http server", logging.Port(httpPort))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server error", zap.Error(err))
		}
	}()

	apiSrv := &server.APIServer{
		DB:       database,
		Domain:   domain,
		PublicIP: publicIP,
		Pepper:   pepperBytes,
		Logger:   logger.Named("api"),
	}

	apiErrLog, _ := zap.NewStdLogAt(logger.Named("api"), zapcore.ErrorLevel)
	apiServer := &http.Server{
		Addr:     fmt.Sprintf(":%d", apiPort),
		Handler:  apiSrv.Handler(),
		ErrorLog: apiErrLog,
	}

	go func() {
		logger.Info("starting api server", logging.Port(apiPort))
		if err := apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("api server error", zap.Error(err))
		}
	}()

	dnsSrv := &server.DNSServer{
		DB:       database,
		Domain:   domain,
		PublicIP: publicIP,
		TXTStore: txtStore,
		Logger:   logger.Named("dns"),
	}
	if err := dnsSrv.Start(dnsPort, dnsPort); err != nil {
		return fmt.Errorf("start DNS server: %w", err)
	}

	var httpsServer *http.Server
	httpsErrLog, _ := zap.NewStdLogAt(logger.Named("https"), zapcore.ErrorLevel)
	if acmeMode {
		// ACME mode: obtain certificates via Let's Encrypt DNS-01 challenge
		manager := acme.NewManager(domain, acmeEmail, database, acmeStaging, txtStore, publicIP, logger.Named("certmagic"))

		logger.Info("starting acme certificate acquisition", logging.Domain(domain), zap.Bool("staging", acmeStaging))
		ctx := context.Background()
		if err := manager.Manage(ctx); err != nil {
			return fmt.Errorf("ACME certificate acquisition: %w", err)
		}
		logger.Info("acme certificate obtained", logging.Domain(domain))

		httpsServer = &http.Server{
			Addr:      fmt.Sprintf(":%d", httpsPort),
			Handler:   httpSrv,
			TLSConfig: manager.TLSConfig(),
			ErrorLog:  httpsErrLog,
		}

		go func() {
			logger.Info("starting https server", logging.Port(httpsPort), logging.TLSMode("acme"))
			if err := httpsServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				logger.Error("https server error", zap.Error(err))
			}
		}()
	} else if manualTLS {
		// Manual TLS mode: use provided certificate files
		cert, err := tls.LoadX509KeyPair(tlsCert, tlsKey)
		if err != nil {
			return fmt.Errorf("load TLS certificate: %w", err)
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
		}

		httpsServer = &http.Server{
			Addr:      fmt.Sprintf(":%d", httpsPort),
			Handler:   httpSrv,
			TLSConfig: tlsConfig,
			ErrorLog:  httpsErrLog,
		}

		go func() {
			logger.Info("starting https server", logging.Port(httpsPort), logging.TLSMode("manual"))
			if err := httpsServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				logger.Error("https server error", zap.Error(err))
			}
		}()
	} else {
		// No HTTPS mode: --no-acme without manual certs
		logger.Info("https disabled", zap.String("reason", "no-acme specified without manual TLS certificates"))
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("shutting down")

	ctx := context.Background()
	if httpsServer != nil {
		httpsServer.Shutdown(ctx)
	}
	httpServer.Shutdown(ctx)
	apiServer.Shutdown(ctx)
	dnsSrv.Shutdown()

	return nil
}

func runGenerate(args []string) error {
	generateCmd := flag.NewFlagSet("generate", flag.ExitOnError)

	var label string
	var apiKey string
	var apiURL string

	generateCmd.StringVar(&label, "label", "", "optional label for the token")
	generateCmd.StringVar(&apiKey, "api-key", os.Getenv("OASTRIX_API_KEY"), "API key for authentication")
	generateCmd.StringVar(&apiURL, "api-url", getEnv("OASTRIX_API_URL", "http://localhost:8081"), "API server URL")

	if err := generateCmd.Parse(args); err != nil {
		return err
	}

	if apiKey == "" {
		return fmt.Errorf("API key required (use --api-key flag or OASTRIX_API_KEY env var)")
	}

	c := client.NewClient(apiURL, apiKey)
	resp, err := c.CreateToken(label)
	if err != nil {
		return err
	}

	fmt.Printf("Token: %s\n", resp.Token)
	fmt.Println()
	fmt.Println("Payloads:")
	if dns, ok := resp.Payloads["dns"]; ok {
		fmt.Printf("  dns:       %s\n", dns)
	}
	if http, ok := resp.Payloads["http"]; ok {
		fmt.Printf("  http:      %s\n", http)
	}
	if https, ok := resp.Payloads["https"]; ok {
		fmt.Printf("  https:     %s\n", https)
	}
	if httpIP, ok := resp.Payloads["http_ip"]; ok {
		fmt.Printf("  http_ip:   %s\n", httpIP)
	}
	if httpsIP, ok := resp.Payloads["https_ip"]; ok {
		fmt.Printf("  https_ip:  %s\n", httpsIP)
	}

	return nil
}

func runInteractions(args []string) error {
	interactionsCmd := flag.NewFlagSet("interactions", flag.ExitOnError)

	var apiKey string
	var apiURL string

	interactionsCmd.StringVar(&apiKey, "api-key", os.Getenv("OASTRIX_API_KEY"), "API key for authentication")
	interactionsCmd.StringVar(&apiURL, "api-url", getEnv("OASTRIX_API_URL", "http://localhost:8081"), "API server URL")

	if err := interactionsCmd.Parse(args); err != nil {
		return err
	}

	if interactionsCmd.NArg() < 1 {
		return fmt.Errorf("usage: oastrix interactions <token>")
	}

	if apiKey == "" {
		return fmt.Errorf("API key required (use --api-key flag or OASTRIX_API_KEY env var)")
	}

	token := interactionsCmd.Arg(0)
	c := client.NewClient(apiURL, apiKey)
	resp, err := c.GetInteractions(token)
	if err != nil {
		return err
	}

	if len(resp.Interactions) == 0 {
		fmt.Println("No interactions found.")
		return nil
	}

	fmt.Printf("%-20s  %-4s  %-16s  %s\n", "TIME", "KIND", "REMOTE", "SUMMARY")
	for _, i := range resp.Interactions {
		t, _ := time.Parse(time.RFC3339, i.OccurredAt)
		timeStr := t.Format("2006-01-02 15:04:05")
		remote := fmt.Sprintf("%s:%d", i.RemoteIP, i.RemotePort)
		fmt.Printf("%-20s  %-4s  %-16s  %s\n", timeStr, i.Kind, remote, i.Summary)
	}

	return nil
}

func runDelete(args []string) error {
	deleteCmd := flag.NewFlagSet("delete", flag.ExitOnError)

	var apiKey string
	var apiURL string

	deleteCmd.StringVar(&apiKey, "api-key", os.Getenv("OASTRIX_API_KEY"), "API key for authentication")
	deleteCmd.StringVar(&apiURL, "api-url", getEnv("OASTRIX_API_URL", "http://localhost:8081"), "API server URL")

	if err := deleteCmd.Parse(args); err != nil {
		return err
	}

	if deleteCmd.NArg() < 1 {
		return fmt.Errorf("usage: oastrix delete <token>")
	}

	if apiKey == "" {
		return fmt.Errorf("API key required (use --api-key flag or OASTRIX_API_KEY env var)")
	}

	token := deleteCmd.Arg(0)
	c := client.NewClient(apiURL, apiKey)
	if err := c.DeleteToken(token); err != nil {
		return err
	}

	fmt.Printf("Token %s deleted.\n", token)
	return nil
}

func runList(args []string) error {
	listCmd := flag.NewFlagSet("list", flag.ExitOnError)

	var apiKey string
	var apiURL string

	listCmd.StringVar(&apiKey, "api-key", os.Getenv("OASTRIX_API_KEY"), "API key for authentication")
	listCmd.StringVar(&apiURL, "api-url", getEnv("OASTRIX_API_URL", "http://localhost:8081"), "API server URL")

	if err := listCmd.Parse(args); err != nil {
		return err
	}

	if apiKey == "" {
		return fmt.Errorf("API key required (use --api-key flag or OASTRIX_API_KEY env var)")
	}

	c := client.NewClient(apiURL, apiKey)
	resp, err := c.ListTokens()
	if err != nil {
		return err
	}

	if len(resp.Tokens) == 0 {
		fmt.Println("No tokens found.")
		return nil
	}

	fmt.Printf("%-14s  %-12s  %-19s  %s\n", "TOKEN", "LABEL", "CREATED", "INTERACTIONS")
	for _, t := range resp.Tokens {
		label := "-"
		if t.Label != nil {
			label = *t.Label
		}
		createdAt, _ := time.Parse(time.RFC3339, t.CreatedAt)
		createdStr := createdAt.Format("2006-01-02 15:04:05")
		fmt.Printf("%-14s  %-12s  %-19s  %d\n", t.Token, label, createdStr, t.InteractionCount)
	}

	return nil
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		var i int
		if _, err := fmt.Sscanf(v, "%d", &i); err == nil {
			return i
		}
	}
	return defaultVal
}

func printUsage() {
	fmt.Println(`oastrix - Out-of-band Application Security Testing (OAST) tool

Usage:
  oastrix <command> [arguments]

Commands:
  server         Start all listeners (HTTP, HTTPS, DNS, API)
  generate       Create a new token
  list           List all tokens with interaction counts
  interactions   List interactions for a token
  delete         Delete a token

Server Flags:
  --http-port    HTTP port (default: 80, env: OASTRIX_HTTP_PORT)
  --https-port   HTTPS port (default: 443, env: OASTRIX_HTTPS_PORT)
  --dns-port     DNS port (default: 53, env: OASTRIX_DNS_PORT)
  --api-port     API port (default: 8081, env: OASTRIX_API_PORT)
  --domain       Domain for token extraction (default: localhost)
  --db           Database path (default: oastrix.db)
  --tls-cert     Path to TLS certificate (enables manual TLS mode)
  --tls-key      Path to TLS key (enables manual TLS mode)
  --no-acme      Disable automatic TLS via Let's Encrypt
  --acme-email   Email for Let's Encrypt notifications
  --acme-staging Use Let's Encrypt staging CA (for testing)

TLS Modes:
  By default, ACME is enabled and certificates are automatically obtained
  from Let's Encrypt using DNS-01 challenges. The DNS server must be
  publicly reachable on port 53 for ACME to work.

  --tls-cert + --tls-key  → Manual TLS mode (use provided certificates)
  --no-acme               → HTTP only (no HTTPS server)
  (neither)               → ACME mode (automatic Let's Encrypt certificates)

Notes:
  Ports 80, 443, and 53 require root or 'setcap cap_net_bind_service'.
  Certificates are stored in <db-dir>/certmagic/.

Use "oastrix <command> -h" for more information about a command.`)
}
