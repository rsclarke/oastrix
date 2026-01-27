package config

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"os"
)

type Config struct {
	DBPath      string
	Pepper      string
	Domain      string
	HTTPPort    int
	HTTPSPort   int
	APIPort     int
	TLSCertFile string
	TLSKeyFile  string
}

func Load() (*Config, error) {
	cfg := &Config{}

	flag.StringVar(&cfg.DBPath, "db", getEnv("OASTRIX_DB", "oastrix.db"), "database path")
	flag.StringVar(&cfg.Pepper, "pepper", os.Getenv("OASTRIX_PEPPER"), "HMAC pepper for API key hashing")
	flag.StringVar(&cfg.Domain, "domain", getEnv("OASTRIX_DOMAIN", "localhost"), "domain for payload URLs")

	if cfg.Pepper == "" {
		pepperBytes := make([]byte, 32)
		if _, err := rand.Read(pepperBytes); err != nil {
			return nil, err
		}
		cfg.Pepper = base64.StdEncoding.EncodeToString(pepperBytes)
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func Default() *Config {
	return &Config{
		DBPath:    "oastrix.db",
		Domain:    "localhost",
		HTTPPort:  8080,
		HTTPSPort: 8443,
		APIPort:   8081,
	}
}
