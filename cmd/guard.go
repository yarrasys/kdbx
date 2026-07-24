package cmd

import (
	"github.com/spf13/cobra"

	"github.com/yarrasys/kdbx/internal/guard"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		var hook string
		cmd := &cobra.Command{
			Use:   "guard",
			Short: "Agent role-guard: decide a PreToolUse hook payload read from stdin",
			Args:  cobra.NoArgs,
			RunE: func(c *cobra.Command, _ []string) error {
				guard.Run(c.InOrStdin(), c.OutOrStdout())
				return nil
			},
		}
		cmd.Flags().StringVar(&hook, "hook", "pretooluse", "hook event to evaluate")
		root.AddCommand(cmd)
	})
}
