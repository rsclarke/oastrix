package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var generateFlags struct {
	clientConfig
	label string
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Create a new token",
	Long:  `Create a new token for out-of-band interaction detection.`,
	RunE:  runGenerate,
}

func init() {
	rootCmd.AddCommand(generateCmd)

	addClientFlags(generateCmd, &generateFlags.clientConfig)
	generateCmd.Flags().StringVar(&generateFlags.label, "label", "", "optional label for the token")
}

func runGenerate(cmd *cobra.Command, args []string) error {
	c, err := generateFlags.newClient()
	if err != nil {
		return err
	}

	resp, err := c.CreateToken(generateFlags.label)
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
