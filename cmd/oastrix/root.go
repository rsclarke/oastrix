package main

import (
	"fmt"
	"os"

	"github.com/rsclarke/oastrix/internal/logging"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var logger *zap.Logger

var rootCmd = &cobra.Command{
	Use:   "oastrix",
	Short: "Out-of-band Application Security Testing (OAST) tool",
	Long: `oastrix is an Out-of-band Application Security Testing (OAST) tool
that provides HTTP, HTTPS, and DNS listeners for detecting out-of-band
interactions during security testing.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		logger, err = logging.New(logging.FromEnv())
		if err != nil {
			return fmt.Errorf("initializing logger: %w", err)
		}
		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if logger != nil {
			logging.Sync(logger)
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
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
