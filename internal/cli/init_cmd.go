package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new vessel.toml",
	Long: `Initialize a new vessel.toml configuration file interactively.

Creates a starter configuration file with your settings.

Examples:
  vessel init                    # Create vessel.toml in current directory
  vessel init --output app.toml  # Create with custom filename`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringP("output", "o", "vessel.toml", "output file path")
}

func runInit(cmd *cobra.Command, args []string) error {
	outputPath, _ := cmd.Flags().GetString("output")

	// Check if file already exists
	if _, err := os.Stat(outputPath); err == nil {
		fmt.Printf("Warning: %s already exists. Overwrite? (y/n): ", outputPath)
		if !promptYesNo() {
			fmt.Println("Aborted.")
			return nil
		}
	}

	reader := bufio.NewReader(os.Stdin)

	// App name
	appName := promptString(reader, "App name", "my-app")

	// Docker image
	image := promptString(reader, "Docker image", "alpine:latest")

	// Port
	portStr := promptString(reader, "Port (leave empty for auto-detect)", "")
	port := 0
	if portStr != "" {
		fmt.Sscanf(portStr, "%d", &port)
	}

	// Hostname
	hostname := promptString(reader, "Hostname for routing (leave empty to skip)", "")

	// Memory
	memory := promptString(reader, "Memory limit (e.g., 256MB, 1GB)", "256MB")

	// Instances
	instancesStr := promptString(reader, "Number of instances", "1")
	instances := 1
	fmt.Sscanf(instancesStr, "%d", &instances)
	if instances < 1 {
		instances = 1
	}

	// Environment variables
	var envVars []struct{ key, value string }
	fmt.Print("? Add environment variables? (y/n): ")
	if promptYesNo() {
		for {
			kv := promptString(reader, "KEY=VALUE (empty to finish)", "")
			if kv == "" {
				break
			}
			eqIdx := strings.IndexByte(kv, '=')
			if eqIdx < 0 {
				fmt.Println("  Invalid format, expected KEY=VALUE")
				continue
			}
			envVars = append(envVars, struct{ key, value string }{
				key:   kv[:eqIdx],
				value: kv[eqIdx+1:],
			})
		}
	}

	// Deploy strategy
	strategy := promptString(reader, "Deploy strategy (rolling/blue-green)", "rolling")
	if strategy != "rolling" && strategy != "blue-green" {
		strategy = "rolling"
	}

	// Build the TOML content
	var b strings.Builder

	b.WriteString("[[app]]\n")
	b.WriteString(fmt.Sprintf("name = %q\n", appName))
	b.WriteString(fmt.Sprintf("image = %q\n", image))
	if instances > 1 {
		b.WriteString(fmt.Sprintf("instances = %d\n", instances))
	}

	if hostname != "" || port > 0 {
		b.WriteString("\n[app.network]\n")
		if hostname != "" {
			b.WriteString(fmt.Sprintf("host = %q\n", hostname))
		}
		if port > 0 {
			b.WriteString(fmt.Sprintf("port = %d\n", port))
		}
	}

	b.WriteString("\n[app.resources]\n")
	b.WriteString(fmt.Sprintf("memory = %q\n", memory))

	b.WriteString("\n[app.deploy]\n")
	b.WriteString(fmt.Sprintf("strategy = %q\n", strategy))

	if len(envVars) > 0 {
		b.WriteString("\n[app.env]\n")
		for _, ev := range envVars {
			b.WriteString(fmt.Sprintf("%s = %q\n", ev.key, ev.value))
		}
	}

	content := b.String()

	// Write the file
	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", outputPath, err)
	}

	fmt.Printf("\nCreated %s:\n", outputPath)
	fmt.Println(strings.Repeat("-", 40))
	fmt.Print(content)
	fmt.Println(strings.Repeat("-", 40))

	fmt.Print("\nDeploy now? (y/n): ")
	if promptYesNo() {
		return runDeploy(cmd, nil)
	}

	return nil
}

// promptString prompts the user for a string value with an optional default.
func promptString(reader *bufio.Reader, prompt string, defaultValue string) string {
	if defaultValue != "" {
		fmt.Printf("? %s [%s]: ", prompt, defaultValue)
	} else {
		fmt.Printf("? %s: ", prompt)
	}

	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)

	if line == "" {
		return defaultValue
	}
	return line
}

// promptYesNo reads a y/n response from stdin.
func promptYesNo() bool {
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes"
}
