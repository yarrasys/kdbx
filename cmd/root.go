// Package cmd wires the kdbx command tree. Command files hold flag plumbing
// only — behavior lives in internal/.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is injected at build time via -ldflags "-X github.com/yarrasys/kdbx/cmd.Version=…".
var Version = "dev"

// Global flags shared by every subcommand.
type globals struct {
	env  string
	json bool
}

var opts globals

// RootCmd builds a fresh command tree. Tests call it to get an isolated root.
func RootCmd() *cobra.Command {
	opts = globals{}
	root := &cobra.Command{
		Use:           "kdbx",
		Short:         "Per-project, per-env KeePassXC credentials",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       Version,
		// Reject stray arguments so a mistyped operation fails loudly instead of
		// silently printing help and exiting 0. RunE must be set for this to take
		// effect: cobra returns help early for a non-runnable command, before it
		// ever validates args.
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error { return c.Help() },
	}
	root.SetVersionTemplate("kdbx {{.Version}}\n")
	root.PersistentFlags().StringVar(&opts.env, "env", "", "environment name (overrides $KDBX_ENV and the pointer default)")
	root.PersistentFlags().BoolVar(&opts.json, "json", false, "machine-readable output (read operations only)")
	register(root)
	return root
}

// register hooks every subcommand onto root. Each cmd/<op>.go appends to
// registrars in its init(); this keeps RootCmd free of a growing import list.
var registrars []func(*cobra.Command)

func register(root *cobra.Command) {
	for _, r := range registrars {
		r(root)
	}
}

// Execute runs the CLI and returns the process exit code.
func Execute() int {
	root := RootCmd()
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "kdbx: %v\n", err)
		return 1
	}
	return 0
}
