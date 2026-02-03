package cli

import "github.com/spf13/cobra"

var secretCmd = &cobra.Command{
	Use:   "secret",
	Short: "Manage secrets",
	Long:  `Manage encrypted secrets for applications.`,
}

var secretSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a secret",
	Long: `Set an encrypted secret value.

Examples:
  vessel secret set DATABASE_URL "postgres://..."
  vessel secret set API_KEY "secret-key"`,
	Args: cobra.ExactArgs(2),
	RunE: notImplemented,
}

var secretGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a secret value",
	Long: `Get and decrypt a secret value.

Examples:
  vessel secret get DATABASE_URL`,
	Args: cobra.ExactArgs(1),
	RunE: notImplemented,
}

var secretListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all secret keys",
	Long: `List all secret keys (not values).

Examples:
  vessel secret list`,
	RunE: notImplemented,
}

var secretDeleteCmd = &cobra.Command{
	Use:   "delete <key>",
	Short: "Delete a secret",
	Long: `Delete a secret.

Examples:
  vessel secret delete API_KEY`,
	Args: cobra.ExactArgs(1),
	RunE: notImplemented,
}

func init() {
	secretCmd.AddCommand(secretSetCmd)
	secretCmd.AddCommand(secretGetCmd)
	secretCmd.AddCommand(secretListCmd)
	secretCmd.AddCommand(secretDeleteCmd)
}
