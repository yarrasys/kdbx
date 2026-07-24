package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		root.AddCommand(&cobra.Command{
			Use:   "completion [bash|zsh|fish|powershell]",
			Short: "Emit a shell completion script",
			// ExactValidArgs is deprecated in cobra ≥1.8; this is its expansion.
			Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
			ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
			RunE: func(c *cobra.Command, args []string) error {
				out := c.OutOrStdout()
				switch args[0] {
				case "bash":
					return c.Root().GenBashCompletionV2(out, true)
				case "zsh":
					return c.Root().GenZshCompletion(out)
				case "fish":
					return c.Root().GenFishCompletion(out, true)
				case "powershell":
					return c.Root().GenPowerShellCompletionWithDesc(out)
				}
				return fmt.Errorf("unsupported shell %q", args[0])
			},
		})
	})
}
