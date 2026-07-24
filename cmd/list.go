package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yarrasys/kdbx/internal/jsonout"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		root.AddCommand(&cobra.Command{
			Use:   "list [GROUP]",
			Short: "List entry paths (never values)",
			Args:  cobra.MaximumNArgs(1),
			RunE: func(c *cobra.Command, args []string) error {
				group := ""
				if len(args) == 1 {
					group = args[0]
				}
				return runList(c, group)
			},
		})
	})
}

func runList(c *cobra.Command, group string) error {
	ctx, err := mustContext(c, false)
	if err != nil {
		return err
	}
	h, err := openVault(ctx)
	if err != nil {
		return err
	}
	defer h.Close()

	all, err := h.ListEntries()
	if err != nil {
		return err
	}
	kept := []string{}
	for _, p := range all {
		if group == "" || strings.HasPrefix(p, group) {
			kept = append(kept, p)
		}
	}
	if opts.json {
		return jsonout.Write(c.OutOrStdout(), map[string]any{"entries": kept})
	}
	for _, p := range kept {
		fmt.Fprintln(c.OutOrStdout(), p)
	}
	return nil
}
