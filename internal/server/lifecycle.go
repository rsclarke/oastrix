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

type ServerConfig struct {
	Addr              string
	Handler           http.Handler
	TLSConfig         *tls.Config
	Logger            *zap.Logger
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
}

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

type ManagedServer struct {
	server   *http.Server
	logger   *zap.Logger
	name     string
	useTLS   bool
	errCh    chan error
	startErr error
}

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

func (m *ManagedServer) Shutdown(ctx context.Context) {
	if m.startErr != nil {
		return
	}
	if err := m.server.Shutdown(ctx); err != nil {
		m.logger.Warn("shutdown error", zap.String("server", m.name), zap.Error(err))
	}
}
