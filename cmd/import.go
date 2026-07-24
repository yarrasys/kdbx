package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/yarrasys/kdbx/internal/dotenv"
	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/pointer"
	"github.com/yarrasys/kdbx/internal/vault"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		root.AddCommand(&cobra.Command{
			Use:   "import FILE",
			Short: "Read an existing .env into the vault and register var mappings",
			Args:  cobra.ExactArgs(1),
			RunE: func(c *cobra.Command, args []string) error {
				return runImport(c, args[0])
			},
		})
	})
}

func runImport(c *cobra.Command, file string) error {
	b, err := os.ReadFile(file)
	if err != nil {
		return kdbxerr.Wrap(err, "NotFound", 2, "reading %s", file)
	}
	vals, order, err := dotenv.Parse(string(b))
	if err != nil {
		return err
	}
	ctx, err := mustContext(c, true)
	if err != nil {
		return err
	}
	for _, name := range order {
		path := "imported/" + name + ":password"
		group, title, field, perr := pointer.ParseEntryPath(path)
		if perr != nil {
			return perr
		}
		if err := vault.SetField(ctx.Vault, ctx.KeyFile, group, title, field, vals[name]); err != nil {
			return err
		}
		ctx.Pointer.SetVar(ctx.Env, name, path)
	}
	if err := ctx.Pointer.Save(); err != nil {
		return err
	}
	fmt.Fprintf(c.ErrOrStderr(),
		"imported %d vars. Reminder: remove/gitignore the source .env; rotate anything ever committed.\n",
		len(order))
	return nil
}
