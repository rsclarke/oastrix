// Package logging provides structured logging configuration.
package logging

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config holds logging configuration options.
type Config struct {
	Level  string // debug|info|warn|error
	Format string // json|console
}

// New creates a new configured zap logger.
func New(cfg Config) (*zap.Logger, error) {
	level := zapcore.InfoLevel
	if cfg.Level != "" {
		if err := level.Set(strings.ToLower(cfg.Level)); err != nil {
			return nil, err
		}
	}

	format := strings.ToLower(cfg.Format)
	if format == "" {
		format = "json"
	}

	var zcfg zap.Config
	if format == "console" {
		zcfg = zap.NewDevelopmentConfig()
	} else {
		zcfg = zap.NewProductionConfig()
	}

	zcfg.Level = zap.NewAtomicLevelAt(level)
	zcfg.EncoderConfig.TimeKey = "ts"
	zcfg.EncoderConfig.LevelKey = "level"
	zcfg.EncoderConfig.MessageKey = "msg"
	zcfg.EncoderConfig.CallerKey = "caller"
	zcfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := zcfg.Build(zap.AddCaller(), zap.AddCallerSkip(0))
	if err != nil {
		return nil, err
	}

	logger = logger.With(zap.String("service", "oastrix"))

	return logger, nil
}

// Sync flushes any buffered log entries.
func Sync(logger *zap.Logger) {
	_ = logger.Sync()
}

// FromEnv creates a Config from environment variables.
func FromEnv() Config {
	return Config{
		Level:  getenv("OASTRIX_LOG_LEVEL", "info"),
		Format: getenv("OASTRIX_LOG_FORMAT", "json"),
	}
}

func getenv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// Component returns a zap field for the component name.
func Component(name string) zap.Field { return zap.String("component", name) }

// Port returns a zap field for the port number.
func Port(port int) zap.Field { return zap.Int("port", port) }

// Addr returns a zap field for an address.
func Addr(addr string) zap.Field { return zap.String("addr", addr) }

// Domain returns a zap field for a domain name.
func Domain(domain string) zap.Field { return zap.String("domain", domain) }

// Token returns a zap field for a token value.
func Token(token string) zap.Field { return zap.String("token", token) }

// RemoteIP returns a zap field for a remote IP address.
func RemoteIP(ip string) zap.Field { return zap.String("remote_ip", ip) }

// RemotePort returns a zap field for a remote port number.
func RemotePort(port int) zap.Field { return zap.Int("remote_port", port) }

// Host returns a zap field for a host name.
func Host(host string) zap.Field { return zap.String("host", host) }

// Method returns a zap field for an HTTP method.
func Method(method string) zap.Field { return zap.String("method", method) }

// Path returns a zap field for a URL path.
func Path(path string) zap.Field { return zap.String("path", path) }

// Scheme returns a zap field for a URL scheme.
func Scheme(scheme string) zap.Field { return zap.String("scheme", scheme) }

// Protocol returns a zap field for a protocol name.
func Protocol(proto string) zap.Field { return zap.String("protocol", proto) }

// Net returns a zap field for a network type.
func Net(net string) zap.Field { return zap.String("net", net) }

// TLSMode returns a zap field for TLS mode.
func TLSMode(mode string) zap.Field { return zap.String("tls_mode", mode) }

// QName returns a zap field for a DNS query name.
func QName(qname string) zap.Field { return zap.String("qname", qname) }

// QType returns a zap field for a DNS query type.
func QType(qtype string) zap.Field { return zap.String("qtype", qtype) }
