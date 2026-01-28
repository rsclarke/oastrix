package main

import (
	"fmt"
	"time"

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
