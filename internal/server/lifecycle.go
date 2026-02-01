package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ServerConfig holds configuration for an HTTP server.
type ServerConfig struct {
	Addr              string
	Handler           http.Handler
	TLSConfig         *tls.Config
	Logger            *zap.Logger
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
}

// DefaultServerConfig returns a ServerConfig with sensible defaults.
func DefaultServerConfig(addr string, handler http.Handler, logger *zap.Logger) ServerConfig {
	return ServerConfig{
		Addr:              addr,
		Handler:           handler,
		Logger:            logger,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
	}
}

// ManagedServer wraps an HTTP server with lifecycle management.
type ManagedServer struct {
	server   *http.Server
	logger   *zap.Logger
	name     string
	useTLS   bool
	errCh    chan error
	startErr error
}

// NewManagedServer creates a new managed HTTP server.
func NewManagedServer(name string, cfg ServerConfig) *ManagedServer {
	errLog, _ := zap.NewStdLogAt(cfg.Logger, zapcore.ErrorLevel)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           cfg.Handler,
		TLSConfig:         cfg.TLSConfig,
		ErrorLog:          errLog,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
	}

	useTLS := cfg.TLSConfig != nil

	return &ManagedServer{
		server: srv,
		logger: cfg.Logger,
		name:   name,
		useTLS: useTLS,
		errCh:  make(chan error, 1),
	}
}

// Start begins listening and serving in a background goroutine.
func (m *ManagedServer) Start() {
	go func() {
		var err error
		if m.useTLS {
			err = m.server.ListenAndServeTLS("", "")
		} else {
			err = m.server.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			m.errCh <- err
		}
		close(m.errCh)
	}()
}

// WaitForStartup waits for the server to start or fail within a timeout.
func (m *ManagedServer) WaitForStartup(timeout time.Duration) error {
	select {
	case err := <-m.errCh:
		if err != nil {
			m.startErr = err
			return fmt.Errorf("%s failed to start: %w", m.name, err)
		}
		return nil
	case <-time.After(timeout):
		return nil
	}
}

// Shutdown gracefully stops the server.
func (m *ManagedServer) Shutdown(ctx context.Context) {
	if m.startErr != nil {
		return
	}
	if err := m.server.Shutdown(ctx); err != nil {
		m.logger.Warn("shutdown error", zap.String("server", m.name), zap.Error(err))
	}
}
