package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vessel/vessel/internal/daemon"
)

var rmCmd = &cobra.Command{
	Use:   "rm <app>",
	Short: "Remove an application",
	Long: `Remove an application and all its containers.

This stops any running containers and removes all state for the app.

Examples:
  vessel rm myapp
  vessel rm myapp --force     # Remove without confirmation`,
	Args: cobra.ExactArgs(1),
	RunE: runRemoveApp,
}

func init() {
	rmCmd.Flags().BoolP("force", "f", false, "force removal without confirmation")
}

func runRemoveApp(cmd *cobra.Command, args []string) error {
	appName := args[0]
	force, _ := cmd.Flags().GetBool("force")

	if !force {
		fmt.Printf("Remove app '%s' and all its containers? [y/N] ", appName)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	client := daemon.NewClient("")
	fmt.Printf("Removing %s... ", appName)

	var result map[string]string
	if err := client.Call("apps.remove", map[string]string{"name": appName}, &result); err != nil {
		fmt.Println("failed")
		return err
	}

	fmt.Println("done")
	return nil
}
