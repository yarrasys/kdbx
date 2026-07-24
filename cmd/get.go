package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/yarrasys/kdbx/internal/jsonout"
	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/pointer"
	"github.com/yarrasys/kdbx/internal/secretio"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		var reveal, clip bool
		cmd := &cobra.Command{
			Use:   "get PATH",
			Short: "Read a secret (masked by default)",
			Args:  cobra.ExactArgs(1),
			RunE: func(c *cobra.Command, args []string) error {
				return runGet(c, args[0], reveal, clip)
			},
		}
		cmd.Flags().BoolVar(&reveal, "reveal", false, "print the value to stdout")
		cmd.Flags().BoolVar(&clip, "clip", false, "copy the value to the clipboard (auto-clears)")
		// Deliberately not MarkFlagsMutuallyExclusive: cobra surfaces that as a
		// generic error, which Execute maps to exit 1. A conflicting flag pair is
		// bad input caught before the vault is ever touched — Preflight/exit 7
		// (spec C6) — so runGet checks it explicitly, like the --json rule.
		root.AddCommand(cmd)
		root.AddCommand(clearClipCmd())
	})
}

func runGet(c *cobra.Command, path string, reveal, clip bool) error {
	if reveal && clip {
		return kdbxerr.Preflight("--reveal and --clip cannot be combined")
	}
	if opts.json && reveal {
		return kdbxerr.Preflight("--json cannot be combined with --reveal")
	}
	group, title, field, err := pointer.ParseEntryPath(path)
	if err != nil {
		return err
	}
	ctx, err := mustContext(c, false)
	if err != nil {
		return err
	}
	h, err := openVault(ctx)
	if err != nil {
		return err
	}
	defer h.Close()

	val, err := h.GetField(group, title, field)
	if err != nil {
		return err
	}

	switch {
	case clip:
		if err := secretio.ClipboardCopy(val, secretio.DefaultClipboardClear); err != nil {
			return err
		}
		fmt.Fprintln(c.ErrOrStderr(), "copied to clipboard (clears shortly)")
	case reveal:
		fmt.Fprintln(c.OutOrStdout(), val)
		fmt.Fprintln(c.ErrOrStderr(), "WARNING: value printed to stdout (scrollback/CI logs)")
	case opts.json:
		return jsonout.Write(c.OutOrStdout(), map[string]any{"path": path, "set": true})
	default:
		fmt.Fprintln(c.OutOrStdout(), secretio.Mask)
	}
	return nil
}

// clearClipCmd is the detached helper ClipboardCopy schedules. It is hidden
// because it is an implementation detail, not part of the CLI contract.
func clearClipCmd() *cobra.Command {
	var after int
	cmd := &cobra.Command{
		Use:    "internal-clear-clip",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return secretio.ClearClipboardAfter(after)
		},
	}
	cmd.Flags().IntVar(&after, "after", 15, "seconds to wait before clearing")
	return cmd
}
