package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/paths"
	"github.com/yarrasys/kdbx/internal/pointer"
	"github.com/yarrasys/kdbx/internal/vault"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		var mode string
		cmd := &cobra.Command{
			Use:   "init [--mode standard|strict]",
			Short: "Create the vault and keyfile for an environment",
			Args:  cobra.NoArgs,
			RunE: func(c *cobra.Command, _ []string) error {
				return runInit(c, mode)
			},
		}
		cmd.Flags().StringVar(&mode, "mode", "",
			"policy mode for this env: standard (default) or strict")
		root.AddCommand(cmd)
	})
}

func runInit(c *cobra.Command, mode string) error {
	switch mode {
	case "", "standard", "strict":
	default:
		return kdbxerr.Preflight("init: unknown --mode %q (want standard or strict)", mode)
	}
	ctx, err := mustContext(c, true)
	if err != nil {
		return err
	}
	if err := vault.Create(ctx.Vault, ctx.KeyFile); err != nil {
		return err
	}
	errOut := c.ErrOrStderr()

	// --mode is chosen at the one moment the human is definitely the one
	// typing (spec N6). It is recorded in the pointer, and for strict the
	// policy hash is anchored into the vault just created, so `run` works
	// immediately without a separate bless.
	if mode != "" {
		ctx.Pointer.SetPolicyMode(ctx.Env, mode)
		if err := ctx.Pointer.Save(); err != nil {
			return err
		}
		fmt.Fprintf(errOut, "policy: %s\n", mode)
		fmt.Fprintf(errOut, "modified tracked file %s — review and commit\n", pointer.Name)
		if mode == "strict" {
			hash, herr := ctx.Pointer.PolicyHash(ctx.Env)
			if herr != nil {
				return herr
			}
			if err := vault.SetCustomData(ctx.Vault, ctx.KeyFile,
				pointer.PolicyAnchorKey(ctx.Env), hash); err != nil {
				return err
			}
			fmt.Fprintf(errOut, "blessed policy for env '%s'\n", ctx.Env)
		}
	}
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
