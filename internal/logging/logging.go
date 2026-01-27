package logging

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Config struct {
	Level  string // debug|info|warn|error
	Format string // json|console
}

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

func Sync(logger *zap.Logger) {
	_ = logger.Sync()
}

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

// Field helpers for consistent keys

func Component(name string) zap.Field   { return zap.String("component", name) }
func Port(port int) zap.Field           { return zap.Int("port", port) }
func Addr(addr string) zap.Field        { return zap.String("addr", addr) }
func Domain(domain string) zap.Field    { return zap.String("domain", domain) }
func Token(token string) zap.Field      { return zap.String("token", token) }
func RemoteIP(ip string) zap.Field      { return zap.String("remote_ip", ip) }
func RemotePort(port int) zap.Field     { return zap.Int("remote_port", port) }
func Host(host string) zap.Field        { return zap.String("host", host) }
func Method(method string) zap.Field    { return zap.String("method", method) }
func Path(path string) zap.Field        { return zap.String("path", path) }
func Scheme(scheme string) zap.Field    { return zap.String("scheme", scheme) }
func Protocol(proto string) zap.Field   { return zap.String("protocol", proto) }
func Net(net string) zap.Field          { return zap.String("net", net) }
func TLSMode(mode string) zap.Field     { return zap.String("tls_mode", mode) }
func QName(qname string) zap.Field      { return zap.String("qname", qname) }
func QType(qtype string) zap.Field      { return zap.String("qtype", qtype) }
