package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/yarrasys/kdbx/internal/jsonout"
	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/pointer"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		root.AddCommand(&cobra.Command{
			Use:   "check",
			Short: "Verify every mapped var resolves",
			Args:  cobra.NoArgs,
			RunE:  runCheck,
		})
	})
}

type missingVar struct {
	Var  string `json:"var"`
	Path string `json:"path"`
}

func runCheck(c *cobra.Command, _ []string) error {
	ctx, err := mustContext(c, false)
	if err != nil {
		return err
	}
	h, err := openVault(ctx)
	if err != nil {
		return err
	}
	defer h.Close()

	missing := []missingVar{}
	for _, name := range ctx.VarOrder {
		path := ctx.Vars[name]
		group, title, field, perr := pointer.ParseEntryPath(path)
		if perr != nil {
			missing = append(missing, missingVar{Var: name, Path: path})
			continue
		}
		if _, gerr := h.GetField(group, title, field); gerr != nil {
			missing = append(missing, missingVar{Var: name, Path: path})
		}
	}

	if opts.json {
		if err := jsonout.Write(c.OutOrStdout(), map[string]any{
			"ok": len(missing) == 0, "missing": missing,
		}); err != nil {
			return err
		}
	} else {
		for _, m := range missing {
			fmt.Fprintf(c.OutOrStdout(), "MISSING %s -> %s\n", m.Var, m.Path)
		}
	}
	if len(missing) > 0 {
		return kdbxerr.Drift("%d mapped var(s) do not resolve", len(missing))
	}
	return nil
}
