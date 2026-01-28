package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var deleteFlags struct {
	clientConfig
}

var deleteCmd = &cobra.Command{
	Use:   "delete <token>",
	Short: "Delete a token",
	Long:  `Delete a token and all its associated interactions.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runDelete,
}

func init() {
	rootCmd.AddCommand(deleteCmd)

	addClientFlags(deleteCmd, &deleteFlags.clientConfig)
}

func runDelete(cmd *cobra.Command, args []string) error {
	c, err := deleteFlags.newClient()
	if err != nil {
		return err
	}

	token := args[0]
	if err := c.DeleteToken(token); err != nil {
		return err
	}

	result := struct {
		Token   string `json:"token"`
		Deleted bool   `json:"deleted"`
	}{
		Token:   token,
		Deleted: true,
	}

	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(b))
	return nil
}
