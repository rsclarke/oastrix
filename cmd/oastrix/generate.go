package main

import (
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

	fmt.Printf("Token: %s\n", resp.Token)
	fmt.Println()
	fmt.Println("Payloads:")
	if dns, ok := resp.Payloads["dns"]; ok {
		fmt.Printf("  dns:       %s\n", dns)
	}
	if http, ok := resp.Payloads["http"]; ok {
		fmt.Printf("  http:      %s\n", http)
	}
	if https, ok := resp.Payloads["https"]; ok {
		fmt.Printf("  https:     %s\n", https)
	}
	if httpIP, ok := resp.Payloads["http_ip"]; ok {
		fmt.Printf("  http_ip:   %s\n", httpIP)
	}
	if httpsIP, ok := resp.Payloads["https_ip"]; ok {
		fmt.Printf("  https_ip:  %s\n", httpsIP)
	}

	return nil
}
