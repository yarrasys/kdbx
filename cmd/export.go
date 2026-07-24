package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/yarrasys/kdbx/internal/dotenv"
	"github.com/yarrasys/kdbx/internal/secretio"
	"github.com/yarrasys/kdbx/internal/vaultvars"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		var out string
		var allowMissing bool
		cmd := &cobra.Command{
			Use:   "export",
			Short: "Render mapped vars as a dotenv file (plaintext — handle with care)",
			Args:  cobra.NoArgs,
			RunE: func(c *cobra.Command, _ []string) error {
				return runExport(c, out, allowMissing)
			},
		}
		cmd.Flags().StringVar(&out, "out", "", "write to this file (0600) instead of stdout")
		cmd.Flags().BoolVar(&allowMissing, "allow-missing", false, "skip vars that do not resolve")
		root.AddCommand(cmd)
	})
}

func runExport(c *cobra.Command, out string, allowMissing bool) error {
	ctx, err := mustContext(c, true)
	if err != nil {
		return err
	}
	vals, order, err := vaultvars.Resolve(ctx, allowMissing)
	if err != nil {
		return err
	}
	text := dotenv.Render(order, vals)
	if out == "" {
		fmt.Fprint(c.OutOrStdout(), text)
		return nil
	}
	fmt.Fprintf(c.ErrOrStderr(),
		"NOTE: ensure %s is gitignored (it holds plaintext secrets)\n", filepath.Base(out))
	if err := secretio.AtomicWriteSecret(out, []byte(text)); err != nil {
		return err
	}
	fmt.Fprintf(c.ErrOrStderr(), "wrote %d vars to %s (0600)\n", len(order), out)
	return nil
}
