package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var interactionsFlags struct {
	clientConfig
}

var interactionsCmd = &cobra.Command{
	Use:   "interactions <token>",
	Short: "List interactions for a token",
	Long:  `List all recorded interactions for a specific token.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runInteractions,
}

func init() {
	rootCmd.AddCommand(interactionsCmd)

	addClientFlags(interactionsCmd, &interactionsFlags.clientConfig)
}

func runInteractions(cmd *cobra.Command, args []string) error {
	c, err := interactionsFlags.newClient()
	if err != nil {
		return err
	}

	token := args[0]
	resp, err := c.GetInteractions(context.Background(), token)
	if err != nil {
		return err
	}

	b, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(b))
	return nil
}
