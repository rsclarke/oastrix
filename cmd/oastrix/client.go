package main

import (
	"fmt"
	"os"

	"github.com/rsclarke/oastrix/internal/client"
	"github.com/spf13/cobra"
)

type clientConfig struct {
	apiKey string
	apiURL string
}

func addClientFlags(cmd *cobra.Command, cfg *clientConfig) {
	cmd.Flags().StringVar(&cfg.apiKey, "api-key", os.Getenv("OASTRIX_API_KEY"), "API key for authentication")
	cmd.Flags().StringVar(&cfg.apiURL, "api-url", getEnv("OASTRIX_API_URL", "http://localhost:8081"), "API server URL")
}

func (cfg *clientConfig) newClient() (*client.Client, error) {
	if cfg.apiKey == "" {
		return nil, fmt.Errorf("API key required (use --api-key flag or OASTRIX_API_KEY env var)")
	}
	return client.NewClient(cfg.apiURL, cfg.apiKey), nil
}
