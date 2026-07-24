package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/pointer"
	"github.com/yarrasys/kdbx/internal/secretio"
	"github.com/yarrasys/kdbx/internal/vault"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		var varName, fromEnv string
		var raw bool
		cmd := &cobra.Command{
			Use:   "set PATH",
			Short: "Store a secret (value via stdin or --from-env, never argv)",
			Args:  cobra.ExactArgs(1),
			RunE: func(c *cobra.Command, args []string) error {
				return runSet(c, args[0], varName, fromEnv, raw)
			},
		}
		cmd.Flags().StringVar(&varName, "var", "", "also map this env-var name to the entry")
		cmd.Flags().StringVar(&fromEnv, "from-env", "", "read the value from this environment variable")
		cmd.Flags().BoolVar(&raw, "raw", false, "do not strip a trailing newline")
		root.AddCommand(cmd)
	})
}

func runSet(c *cobra.Command, path, varName, fromEnv string, raw bool) error {
	if varName != "" && !pointer.ValidVarName(varName) {
		fmt.Fprintf(c.ErrOrStderr(),
			"kdbx set: --var '%s' is not a valid env-var name (expected pattern: ^[A-Z_][A-Z0-9_]*$)\n",
			varName)
		return kdbxerr.Preflight("invalid --var name")
	}
	group, title, field, err := pointer.ParseEntryPath(path)
	if err != nil {
		return err
	}
	ctx, err := mustContext(c, true)
	if err != nil {
		return err
	}
	value, err := secretio.ReadSecret(secretio.ReadOpts{
		FromEnv: fromEnv,
		Raw:     raw,
		Stdin:   c.InOrStdin(),
		IsTTY:   fromEnv == "" && secretio.IsTerminal(os.Stdin),
	})
	if err != nil {
		return err
	}
	if err := vault.SetField(ctx.Vault, ctx.KeyFile, group, title, field, value); err != nil {
		return err
	}
	if varName != "" {
		ctx.Pointer.SetVar(ctx.Env, varName, path)
		if err := ctx.Pointer.Save(); err != nil {
			return err
		}
		fmt.Fprintf(c.ErrOrStderr(),
			"modified tracked file %s — review and commit\n", pointer.Name)
	}
	return nil
}
