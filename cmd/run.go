package cmd

import (
	"github.com/spf13/cobra"

	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/runner"
	"github.com/yarrasys/kdbx/internal/vaultvars"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		var allowMissing bool
		cmd := &cobra.Command{
			Use:   "run [--allow-missing] -- CMD [ARGS...]",
			Short: "Inject this environment's secrets and exec a command",
			RunE: func(c *cobra.Command, args []string) error {
				return runRun(c, args, allowMissing)
			},
		}
		cmd.Flags().BoolVar(&allowMissing, "allow-missing", false, "skip vars that do not resolve")
		cmd.Flags().SetInterspersed(false)
		root.AddCommand(cmd)
	})
}

// runExitCode carries a child's exit status out through cobra's error path
// without printing a failure line — the child already reported for itself.
type runExitCode struct{ code int }

func (e *runExitCode) Error() string { return "" }

func runRun(c *cobra.Command, args []string, allowMissing bool) error {
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
	vals, _, err := vaultvars.Resolve(ctx, allowMissing)
	if err != nil {
		return err
	}
	code, err := runner.Run(args, vals, c.InOrStdin(), c.OutOrStdout(), c.ErrOrStderr())
	if err != nil {
		return err
	}
	if code != 0 {
		return &runExitCode{code: code}
	}
	return nil
}
