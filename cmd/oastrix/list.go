package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var listFlags struct {
	clientConfig
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tokens with interaction counts",
	Long:  `List all tokens with their labels, creation times, and interaction counts.`,
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)

	addClientFlags(listCmd, &listFlags.clientConfig)
}

func runList(cmd *cobra.Command, args []string) error {
	c, err := listFlags.newClient()
	if err != nil {
		return err
	}

	resp, err := c.ListTokens()
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
