package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/yarrasys/kdbx/internal/paths"
	"github.com/yarrasys/kdbx/internal/vault"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		root.AddCommand(&cobra.Command{
			Use:   "init",
			Short: "Create the vault and keyfile for an environment",
			Args:  cobra.NoArgs,
			RunE:  runInit,
		})
	})
}

func runInit(c *cobra.Command, _ []string) error {
	ctx, err := mustContext(c, true)
	if err != nil {
		return err
	}
	if err := vault.Create(ctx.Vault, ctx.KeyFile); err != nil {
		return err
	}
	errOut := c.ErrOrStderr()
	fmt.Fprintf(errOut, "created %s\n", ctx.Vault)
	fmt.Fprintf(errOut,
		"KEYFILE: %s — back this up; losing it makes the vault unrecoverable.\n", ctx.KeyFile)
	for _, p := range []string{ctx.Vault, ctx.KeyFile} {
		if root := paths.UnderSyncRoot(p); root != "" {
			fmt.Fprintf(errOut,
				"WARNING: %s is inside %s — cloud sync can corrupt a vault and copies the keyfile off this machine.\n",
				p, root)
			break
		}
	}
	return nil
}
