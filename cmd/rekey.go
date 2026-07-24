package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/secretio"
	"github.com/yarrasys/kdbx/internal/vault"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		var yes bool
		cmd := &cobra.Command{
			Use:   "rekey",
			Short: "Rotate the environment's keyfile",
			Args:  cobra.NoArgs,
			RunE: func(c *cobra.Command, _ []string) error {
				return runRekey(c, yes)
			},
		}
		cmd.Flags().BoolVar(&yes, "yes", false, "test-only: bypass the interactive confirmation")
		_ = cmd.Flags().MarkHidden("yes")
		root.AddCommand(cmd)
	})
}

func runRekey(c *cobra.Command, yes bool) error {
	ctx, err := mustContext(c, true)
	if err != nil {
		return err
	}
	if !yes {
		ok := secretio.Confirm(
			fmt.Sprintf("rotate the key file for env '%s'? the old key file is deleted", ctx.Env),
			c.InOrStdin(), c.ErrOrStderr(), secretio.IsTerminal(os.Stdin))
		if !ok {
			return kdbxerr.NotConfirmed("rekey not confirmed")
		}
	}
	newKey := ctx.KeyFile + ".new"
	if err := vault.Rekey(ctx.Vault, ctx.KeyFile, newKey); err != nil {
		return err
	}
	if err := os.Rename(newKey, ctx.KeyFile); err != nil {
		return kdbxerr.Wrap(err, "Runtime", 1, "installing the new keyfile")
	}
	fmt.Fprintln(c.ErrOrStderr(),
		"rekeyed. A prior keyfile+vault leak means secrets are already exposed — rotate at source.")
	return nil
}
