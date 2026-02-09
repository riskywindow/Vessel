package cli

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion script",
	Long: `Generate shell completion scripts for Vessel.

To load completions:

Bash:
  $ source <(vessel completion bash)
  # To load completions for each session, execute once:
  # Linux:
  $ vessel completion bash > /etc/bash_completion.d/vessel
  # macOS:
  $ vessel completion bash > $(brew --prefix)/etc/bash_completion.d/vessel

Zsh:
  $ source <(vessel completion zsh)
  # To load completions for each session, execute once:
  $ vessel completion zsh > "${fpath[1]}/_vessel"

Fish:
  $ vessel completion fish | source
  # To load completions for each session, execute once:
  $ vessel completion fish > ~/.config/fish/completions/vessel.fish

PowerShell:
  PS> vessel completion powershell | Out-String | Invoke-Expression
  # To load completions for each session, execute once:
  PS> vessel completion powershell > vessel.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}
