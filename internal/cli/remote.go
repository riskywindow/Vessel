package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"github.com/vessel/vessel/internal/cli/styles"
	"github.com/vessel/vessel/internal/config"
)

var remoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "Manage remote servers",
	Long: `Manage remote server configurations for SSH deployment.

Remotes are stored in vessel.toml as [[remote]] sections.

Examples:
  vessel remote list                           # List configured remotes
  vessel remote add prod root@prod.example.com # Add a remote
  vessel remote add staging deploy@staging -i ~/.ssh/staging_key
  vessel remote remove prod                    # Remove a remote`,
}

var remoteListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured remotes",
	RunE:  runRemoteList,
}

var remoteAddCmd = &cobra.Command{
	Use:   "add <name> <user@host>",
	Short: "Add a remote server",
	Long: `Add a remote server configuration to vessel.toml.

Examples:
  vessel remote add prod root@prod.example.com
  vessel remote add staging deploy@staging.example.com -p 2222
  vessel remote add dev admin@192.168.1.100 -i ~/.ssh/dev_key`,
	Args: cobra.ExactArgs(2),
	RunE: runRemoteAdd,
}

var remoteRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a remote server",
	Args:  cobra.ExactArgs(1),
	RunE:  runRemoteRemove,
}

var (
	remoteAddPort    int
	remoteAddKeyFile string
)

func init() {
	remoteAddCmd.Flags().IntVarP(&remoteAddPort, "port", "p", 22, "SSH port")
	remoteAddCmd.Flags().StringVarP(&remoteAddKeyFile, "identity", "i", "", "path to SSH private key")

	remoteCmd.AddCommand(remoteListCmd)
	remoteCmd.AddCommand(remoteAddCmd)
	remoteCmd.AddCommand(remoteRemoveCmd)
}

func runRemoteList(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.Remotes) == 0 {
		fmt.Println("No remotes configured.")
		fmt.Println("Add one with: vessel remote add <name> <user@host>")
		return nil
	}

	if jsonOutput {
		data, err := json.MarshalIndent(cfg.Remotes, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("%-12s %-12s %-30s %-6s %-20s\n",
		styles.TableHeader.Render("NAME"),
		styles.TableHeader.Render("USER"),
		styles.TableHeader.Render("HOST"),
		styles.TableHeader.Render("PORT"),
		styles.TableHeader.Render("KEY"))
	fmt.Println(strings.Repeat("-", 82))

	for _, r := range cfg.Remotes {
		port := r.Port
		if port == 0 {
			port = 22
		}
		keyFile := r.KeyFile
		if keyFile == "" {
			keyFile = "(default)"
		}
		fmt.Printf("%-12s %-12s %-30s %-6d %-20s\n",
			styles.AppName.Render(r.Name),
			r.User,
			r.Host,
			port,
			keyFile)
	}

	return nil
}

func runRemoteAdd(cmd *cobra.Command, args []string) error {
	name := args[0]
	target := args[1]

	// Parse user@host
	var user, host string
	if strings.Contains(target, "@") {
		parts := strings.SplitN(target, "@", 2)
		user = parts[0]
		host = parts[1]
	} else {
		host = target
		user = os.Getenv("USER")
	}

	// Load existing config
	cfg, err := config.Load(configFile)
	if err != nil {
		// If config doesn't exist, create a new one
		cfg = &config.Config{}
	}

	// Check for duplicate name
	for _, r := range cfg.Remotes {
		if r.Name == name {
			return fmt.Errorf("remote %q already exists — use 'vessel remote remove %s' first", name, name)
		}
	}

	// Add the new remote
	remote := config.RemoteConfig{
		Name:    name,
		Host:    host,
		User:    user,
		Port:    remoteAddPort,
		KeyFile: remoteAddKeyFile,
	}
	cfg.Remotes = append(cfg.Remotes, remote)

	// Write back to config file
	if err := writeRemotesToConfig(configFile, cfg.Remotes); err != nil {
		return fmt.Errorf("failed to update config: %w", err)
	}

	fmt.Printf("%s Added remote %s (%s@%s:%d)\n",
		styles.SuccessIcon(),
		styles.AppName.Render(name),
		user, host, remoteAddPort)
	return nil
}

func runRemoteRemove(cmd *cobra.Command, args []string) error {
	name := args[0]

	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	found := false
	var remaining []config.RemoteConfig
	for _, r := range cfg.Remotes {
		if r.Name == name {
			found = true
			continue
		}
		remaining = append(remaining, r)
	}

	if !found {
		return fmt.Errorf("remote %q not found", name)
	}

	cfg.Remotes = remaining

	if err := writeRemotesToConfig(configFile, cfg.Remotes); err != nil {
		return fmt.Errorf("failed to update config: %w", err)
	}

	fmt.Printf("%s Removed remote %s\n",
		styles.SuccessIcon(),
		styles.AppName.Render(name))
	return nil
}

// writeRemotesToConfig appends/updates [[remote]] sections in the config file.
func writeRemotesToConfig(path string, remotes []config.RemoteConfig) error {
	// Read existing file content
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Remove existing [[remote]] sections
	content := string(existing)
	lines := strings.Split(content, "\n")
	var filtered []string
	inRemote := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[[remote]]" {
			inRemote = true
			continue
		}
		if inRemote {
			// Check if we've entered a new section
			if strings.HasPrefix(trimmed, "[[") || strings.HasPrefix(trimmed, "[") {
				inRemote = false
				filtered = append(filtered, line)
			}
			continue
		}
		filtered = append(filtered, line)
	}

	// Remove trailing empty lines
	for len(filtered) > 0 && strings.TrimSpace(filtered[len(filtered)-1]) == "" {
		filtered = filtered[:len(filtered)-1]
	}

	// Build the new content
	var builder strings.Builder
	builder.WriteString(strings.Join(filtered, "\n"))

	// Append remote sections
	if len(remotes) > 0 {
		builder.WriteString("\n")
		for _, r := range remotes {
			builder.WriteString("\n[[remote]]\n")
			// Use TOML encoder for each remote
			if err := toml.NewEncoder(&builder).Encode(r); err != nil {
				return err
			}
		}
	}

	return os.WriteFile(path, []byte(builder.String()), 0644)
}
