package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/pointer"
	"github.com/yarrasys/kdbx/internal/secretio"
	"github.com/yarrasys/kdbx/internal/vault"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		var purge, yes bool
		cmd := &cobra.Command{
			Use:   "delete PATH",
			Short: "Soft-delete an entry to the Recycle Bin (--purge removes it permanently)",
			Args:  cobra.ExactArgs(1),
			RunE: func(c *cobra.Command, args []string) error {
				return runDelete(c, args[0], purge, yes)
			},
		}
		cmd.Flags().BoolVar(&purge, "purge", false, "permanently remove the entry")
		cmd.Flags().BoolVar(&yes, "yes", false, "test-only: bypass the interactive confirmation")
		_ = cmd.Flags().MarkHidden("yes")
		root.AddCommand(cmd)
	})
}

func runDelete(c *cobra.Command, path string, purge, yes bool) error {
	group, title, _, err := pointer.ParseEntryPath(path)
	if err != nil {
		return err
	}
	ctx, err := mustContext(c, true)
	if err != nil {
		return err
	}
	if purge && !yes {
		ok := secretio.Confirm(
			"permanently purge '"+path+"'? this cannot be undone",
			c.InOrStdin(), c.ErrOrStderr(), secretio.IsTerminal(os.Stdin))
		if !ok {
			return kdbxerr.NotConfirmed("purge not confirmed")
		}
	}
	if purge {
		return vault.Purge(ctx.Vault, ctx.KeyFile, group, title)
	}
	return vault.Trash(ctx.Vault, ctx.KeyFile, group, title)
}
