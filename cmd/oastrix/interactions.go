package main

import (
	"fmt"
	"time"

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
	resp, err := c.GetInteractions(token)
	if err != nil {
		return err
	}

	if len(resp.Interactions) == 0 {
		fmt.Println("No interactions found.")
		return nil
	}

	fmt.Printf("%-20s  %-4s  %-16s  %s\n", "TIME", "KIND", "REMOTE", "SUMMARY")
	for _, i := range resp.Interactions {
		t, _ := time.Parse(time.RFC3339, i.OccurredAt)
		timeStr := t.Format("2006-01-02 15:04:05")
		remote := fmt.Sprintf("%s:%d", i.RemoteIP, i.RemotePort)
		fmt.Printf("%-20s  %-4s  %-16s  %s\n", timeStr, i.Kind, remote, i.Summary)
	}

	return nil
}
