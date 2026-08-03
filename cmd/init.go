package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/paths"
	"github.com/yarrasys/kdbx/internal/pointer"
	"github.com/yarrasys/kdbx/internal/secretio"
	"github.com/yarrasys/kdbx/internal/vault"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		var mode string
		var here, yes bool
		cmd := &cobra.Command{
			Use:   "init [--here] [--mode standard|strict]",
			Short: "Create the vault and keyfile for an environment",
			Args:  cobra.NoArgs,
			RunE: func(c *cobra.Command, _ []string) error {
				return runInit(c, mode, here, yes)
			},
		}
		cmd.Flags().StringVar(&mode, "mode", "",
			"policy mode for this env: standard (default) or strict")
		cmd.Flags().BoolVar(&here, "here", false,
			"start a new project in this directory, even inside another kdbx project")
		cmd.Flags().BoolVar(&yes, "yes", false, "test-only: bypass the interactive confirmation")
		_ = cmd.Flags().MarkHidden("yes")
		root.AddCommand(cmd)
	})
}

func runInit(c *cobra.Command, mode string, here, yes bool) error {
	switch mode {
	case "", "standard", "strict":
	default:
		return kdbxerr.Preflight("init: unknown --mode %q (want standard or strict)", mode)
	}
	errOut := c.ErrOrStderr()
	cwd, err := os.Getwd()
	if err != nil {
		return kdbxerr.Wrap(err, "Runtime", 1, "getting working directory")
	}

	// Bootstrap a pointer when asked (--here), or when there is none anywhere
	// above — that path could only ever fail before, so claiming it changes
	// nothing that worked (spec C5). Discovery walking up to a *parent*
	// project is legitimate (a cloned repo with a committed pointer), so that
	// case keeps its meaning and instead announces itself below.
	if here {
		if _, err := pointer.Bootstrap(cwd); err != nil {
			return err
		}
		fmt.Fprintf(errOut, "created %s — review and commit\n", pointer.Name)
	} else if _, ferr := pointer.Find(cwd); ferr != nil {
		if _, err := pointer.Bootstrap(cwd); err != nil {
			return err
		}
		fmt.Fprintf(errOut, "created %s — review and commit\n", pointer.Name)
	}

	ctx, err := mustContext(c, true)
	if err != nil {
		return err
	}
	// Binding to a pointer in a parent directory is the one ambiguous case:
	// legitimate for a cloned repo with a committed pointer, a trap when the
	// user meant to start a project here. Name the pointer being used and
	// confirm before creating anything; no TTY means no consent, same as
	// rekey (exit 4).
	if !here && ctx.Pointer.Dir() != paths.Resolve(cwd) {
		fmt.Fprintf(errOut,
			"note: %s has no pointer of its own; this uses %s and initializes env '%s' of project '%s'.\n"+
				"To start a separate project in this directory, run: kdbx init --here\n",
			cwd, filepath.Join(ctx.Pointer.Dir(), pointer.Name), ctx.Env, ctx.Pointer.Project())
		if !yes {
			ok := secretio.Confirm(
				fmt.Sprintf("create env '%s' of project '%s'?", ctx.Env, ctx.Pointer.Project()),
				c.InOrStdin(), errOut, secretio.IsTerminal(os.Stdin))
			if !ok {
				return kdbxerr.NotConfirmed("init not confirmed")
			}
		}
	}
	if err := vault.Create(ctx.Vault, ctx.KeyFile); err != nil {
		return err
	}

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
