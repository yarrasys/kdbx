package cmd

import (
	"io"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/yarrasys/kdbx/internal/allowlist"
	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/maskio"
	"github.com/yarrasys/kdbx/internal/runner"
	"github.com/yarrasys/kdbx/internal/vaultvars"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		var allowMissing, noMask, anyCmd bool
		cmd := &cobra.Command{
			Use:   "run [--allow-missing] [--no-mask] [--any] -- CMD [ARGS...]",
			Short: "Inject this environment's secrets and exec a command",
			RunE: func(c *cobra.Command, args []string) error {
				return runRun(c, args, allowMissing, noMask, anyCmd)
			},
		}
		cmd.Flags().BoolVar(&allowMissing, "allow-missing", false, "skip vars that do not resolve")
		cmd.Flags().BoolVar(&noMask, "no-mask", false,
			"do not mask injected values in captured child output")
		cmd.Flags().BoolVar(&anyCmd, "any", false,
			"run a command that is not in the pointer's run.allow list")
		cmd.Flags().SetInterspersed(false)
		root.AddCommand(cmd)
	})
}

// runExitCode carries a child's exit status out through cobra's error path
// without printing a failure line — the child already reported for itself.
type runExitCode struct{ code int }

func (e *runExitCode) Error() string { return "" }

func runRun(c *cobra.Command, args []string, allowMissing, noMask, anyCmd bool) error {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		return kdbxerr.NotFound("kdbx run: no command given (use: run -- <cmd> ...)")
	}
	ctx, err := mustContext(c, true)
	if err != nil {
		return err
	}

	// The pointer's run.allow list pins which commands may receive this
	// environment's values (spec C5). Checked before the vault is touched:
	// a refused command has no business opening it. --any bypasses for ad hoc
	// human use; the guard denies --any for agents (N3).
	if ctx.AllowSet && !anyCmd {
		ok, err := allowlist.Match(ctx.Allow, args)
		if err != nil {
			return err
		}
		if !ok {
			return kdbxerr.NotAllowed(
				"kdbx run: command not in this env's run.allow list " +
					"(add it to .keepassxc.json, or a human can pass --any)")
		}
	}

	vals, _, err := vaultvars.Resolve(ctx, allowMissing)
	if err != nil {
		return err
	}

	// Captured output gets the injected values masked (spec C5): a pipe means
	// the bytes are headed for a transcript, a log, or a variable, and none of
	// those need the real values. A terminal keeps the raw fd, so interactive
	// children are untouched. --no-mask restores raw output for humans piping
	// binary data; the guard denies it for agents (N3).
	stdout, stderr := c.OutOrStdout(), c.ErrOrStderr()
	var masks []*maskio.Writer
	if !noMask {
		if values := maskio.Values(vals); len(values) > 0 {
			if !isTerminal(stdout) {
				m := maskio.New(stdout, values)
				stdout, masks = m, append(masks, m)
			}
			if !isTerminal(stderr) {
				m := maskio.New(stderr, values)
				stderr, masks = m, append(masks, m)
			}
		}
	}
	defer func() {
		// Flush the held-back tail on every path. A flush failure means the
		// destination is gone (closed pipe); output is lost either way, and
		// losing bytes errs toward masking, never away from it.
		for _, m := range masks {
			_ = m.Flush()
		}
	}()

	code, err := runner.Run(args, vals, c.InOrStdin(), stdout, stderr)
	if err != nil {
		return err
	}
	if code != 0 {
		return &runExitCode{code: code}
	}
	return nil
}

// isTerminal reports whether w is a real terminal fd, in which case the child
// keeps it untouched (wrapping would swap the console for a pipe and break
// interactive children).
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}
