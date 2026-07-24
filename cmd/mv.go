package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/yarrasys/kdbx/internal/pointer"
	"github.com/yarrasys/kdbx/internal/vault"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		root.AddCommand(&cobra.Command{
			Use:   "mv SRC DST",
			Short: "Rename or move an entry, repointing affected var mappings",
			Args:  cobra.ExactArgs(2),
			RunE: func(c *cobra.Command, args []string) error {
				return runMv(c, args[0], args[1])
			},
		})
	})
}

func runMv(c *cobra.Command, src, dst string) error {
	ctx, err := mustContext(c, true)
	if err != nil {
		return err
	}
	if err := vault.Move(ctx.Vault, ctx.KeyFile, src, dst); err != nil {
		return err
	}
	srcEntry, dstEntry := pointer.EntryOf(src), pointer.EntryOf(dst)
	if n := ctx.Pointer.RepointVars(ctx.Env, srcEntry, dstEntry); n > 0 {
		if err := ctx.Pointer.Save(); err != nil {
			return err
		}
		fmt.Fprintf(c.ErrOrStderr(),
			"re-pointed %d var mapping(s) %s -> %s in %s — review and commit\n",
			n, srcEntry, dstEntry, pointer.Name)
	}
	return nil
}
