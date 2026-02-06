package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/vessel/vessel/internal/daemon"
	"github.com/vessel/vessel/internal/store"
)

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
	RunE: runSecretSet,
}

var secretGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a secret value",
	Long: `Get and decrypt a secret value.

Examples:
  vessel secret get DATABASE_URL`,
	Args: cobra.ExactArgs(1),
	RunE: runSecretGet,
}

var secretListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all secret keys",
	Long: `List all secret keys (not values).

Examples:
  vessel secret list`,
	RunE: runSecretList,
}

var secretDeleteCmd = &cobra.Command{
	Use:   "rm <key>",
	Short: "Delete a secret",
	Long: `Delete a secret.

Examples:
  vessel secret rm API_KEY`,
	Args:    cobra.ExactArgs(1),
	Aliases: []string{"delete"},
	RunE:    runSecretDelete,
}

func init() {
	secretCmd.AddCommand(secretSetCmd)
	secretCmd.AddCommand(secretGetCmd)
	secretCmd.AddCommand(secretListCmd)
	secretCmd.AddCommand(secretDeleteCmd)
}

func runSecretSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	client := daemon.NewClient("")

	var result map[string]string
	err := client.Call("secrets.set", map[string]string{
		"key":   key,
		"value": value,
	}, &result)
	if err != nil {
		return fmt.Errorf("failed to set secret: %w", err)
	}

	fmt.Printf("Secret %q set successfully\n", key)
	return nil
}

func runSecretGet(cmd *cobra.Command, args []string) error {
	key := args[0]

	client := daemon.NewClient("")

	var result struct {
		Value string `json:"value"`
	}
	err := client.Call("secrets.get", map[string]string{"key": key}, &result)
	if err != nil {
		return fmt.Errorf("failed to get secret: %w", err)
	}

	if jsonOutput {
		data, _ := json.MarshalIndent(map[string]string{
			"key":   key,
			"value": result.Value,
		}, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Println(result.Value)
	return nil
}

func runSecretList(cmd *cobra.Command, args []string) error {
	client := daemon.NewClient("")

	var secrets []store.Secret
	err := client.Call("secrets.list", nil, &secrets)
	if err != nil {
		return fmt.Errorf("failed to list secrets: %w", err)
	}

	if jsonOutput {
		data, _ := json.MarshalIndent(secrets, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	if len(secrets) == 0 {
		fmt.Println("No secrets found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "KEY\tCREATED\tUPDATED")
	for _, s := range secrets {
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			s.Key,
			timeAgo(s.CreatedAt),
			timeAgo(s.UpdatedAt),
		)
	}
	w.Flush()

	return nil
}

func runSecretDelete(cmd *cobra.Command, args []string) error {
	key := args[0]

	client := daemon.NewClient("")

	var result map[string]string
	err := client.Call("secrets.delete", map[string]string{"key": key}, &result)
	if err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	fmt.Printf("Secret %q deleted\n", key)
	return nil
}

// timeAgo formats a time as a human-readable relative time string.
func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}
