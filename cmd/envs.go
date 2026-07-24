package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/yarrasys/kdbx/internal/jsonout"
	"github.com/yarrasys/kdbx/internal/pointer"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		root.AddCommand(&cobra.Command{
			Use:   "envs",
			Short: "List configured environments and mark the active one",
			Args:  cobra.NoArgs,
			RunE: func(c *cobra.Command, _ []string) error {
				return runEnvs(c)
			},
		})
	})
}

type envRow struct {
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

// runEnvs deliberately stops at the pointer file: listing environments must
// work even when an env's vault or key file cannot be resolved, so it does not
// go through envctx.Resolve.
func runEnvs(c *cobra.Command) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	path, err := pointer.Find(cwd)
	if err != nil {
		return err
	}
	p, err := pointer.Load(path)
	if err != nil {
		return err
	}
	active, source := p.SelectEnv(opts.env)

	if opts.json {
		rows := []envRow{}
		for _, name := range p.EnvNames() {
			rows = append(rows, envRow{Name: name, Active: name == active})
		}
		return jsonout.Write(c.OutOrStdout(), map[string]any{"envs": rows, "source": source})
	}
	for _, name := range p.EnvNames() {
		marker := "  "
		if name == active {
			marker = "* "
		}
		fmt.Fprintf(c.OutOrStdout(), "%s%s\n", marker, name)
	}
	fmt.Fprintf(c.ErrOrStderr(), "active: %s (source: %s)\n", active, source)
	return nil
}
