package cli

import (
	"github.com/spf13/cobra"
	"github.com/vessel/vessel/internal/daemon"
	"github.com/vessel/vessel/internal/store"
)

// completeAppNames provides dynamic completion for app names by querying the daemon.
func completeAppNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	client := daemon.NewClient("")
	var apps []*store.App
	if err := client.Call("apps.list", nil, &apps); err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var names []string
	for _, a := range apps {
		names = append(names, a.Name)
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}
