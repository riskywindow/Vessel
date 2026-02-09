package cli

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "Manage API settings",
	Long:  `Commands for managing the Vessel REST API configuration.`,
}

var apiKeyGenerateCmd = &cobra.Command{
	Use:   "key-generate",
	Short: "Generate a new API key",
	Long: `Generate a cryptographically secure API key for authenticating
with the Vessel REST API.

Example:
  vessel api key-generate
  vessel api key-generate --json`,
	RunE: runAPIKeyGenerate,
}

func init() {
	apiCmd.AddCommand(apiKeyGenerateCmd)
}

func runAPIKeyGenerate(cmd *cobra.Command, args []string) error {
	// Generate 32 random bytes
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return fmt.Errorf("failed to generate key: %w", err)
	}

	// Encode as URL-safe base64
	encoded := base64.URLEncoding.EncodeToString(key)

	if jsonOutput {
		data, _ := json.MarshalIndent(map[string]string{"api_key": encoded}, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Generated API key:\n\n  %s\n\n", encoded)
	fmt.Println("Add this to your vessel.toml:")
	fmt.Printf("\n  [api]\n  enabled = true\n  api_keys = [\"%s\"]\n\n", encoded)

	return nil
}
