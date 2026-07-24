package cmd

import (
	"github.com/spf13/cobra"

	"github.com/yarrasys/kdbx/internal/mcpserver"
)

func init() {
	registrars = append(registrars, func(root *cobra.Command) {
		root.AddCommand(&cobra.Command{
			Use:   "mcp",
			Short: "Run a read-only MCP server over stdio",
			Args:  cobra.NoArgs,
			RunE: func(c *cobra.Command, _ []string) error {
				return mcpserver.Serve(c.Context())
			},
		})
	})
}
