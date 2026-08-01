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
		policyCmd := &cobra.Command{
			Use:   "policy",
			Short: "Manage the environment's run policy (spec N6)",
		}
		var yes bool
		bless := &cobra.Command{
			Use:   "bless",
			Short: "Anchor the pointer's current policy into the vault",
			Args:  cobra.NoArgs,
			RunE: func(c *cobra.Command, _ []string) error {
				return runBless(c, yes)
			},
		}
		bless.Flags().BoolVar(&yes, "yes", false, "test-only: bypass the interactive confirmation")
		_ = bless.Flags().MarkHidden("yes")
		policyCmd.AddCommand(bless)
		root.AddCommand(policyCmd)
	})
}

// runBless is the human approval act the strict policy hinges on: it records
// a hash of the pointer's policy inside the vault, which only human-role
// operations write. Interactive-only, like rekey — an agent harness has no
// TTY to confirm on, and the guard denies the command outright.
func runBless(c *cobra.Command, yes bool) error {
	ctx, err := mustContext(c, true)
	if err != nil {
		return err
	}
	if !yes {
		ok := secretio.Confirm(
			fmt.Sprintf("anchor the current policy for env '%s' into the vault?", ctx.Env),
			c.InOrStdin(), c.ErrOrStderr(), secretio.IsTerminal(os.Stdin))
		if !ok {
			return kdbxerr.NotConfirmed("bless not confirmed")
		}
	}
	hash, err := ctx.Pointer.PolicyHash(ctx.Env)
	if err != nil {
		return err
	}
	if err := vault.SetCustomData(ctx.Vault, ctx.KeyFile,
		pointer.PolicyAnchorKey(ctx.Env), hash); err != nil {
		return err
	}
	fmt.Fprintf(c.ErrOrStderr(), "blessed policy for env '%s'\n", ctx.Env)
	return nil
}
