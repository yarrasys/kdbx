// Package mcpserver exposes kdbx's read-only operations over MCP (spec N2). The
// role contract applies to machines too: there are no write tools, and no tool
// ever returns a secret value.
package mcpserver

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/yarrasys/kdbx/internal/envctx"
	"github.com/yarrasys/kdbx/internal/pointer"
	"github.com/yarrasys/kdbx/internal/runner"
	"github.com/yarrasys/kdbx/internal/secretio"
	"github.com/yarrasys/kdbx/internal/vault"
	"github.com/yarrasys/kdbx/internal/vaultvars"
)

// ToolSpec is one MCP tool, kept transport-agnostic so it is directly testable.
type ToolSpec struct {
	Name        string
	Description string
	Handler     func(ctx context.Context, args map[string]any) (string, error)
}

func str(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func currentContext() (*envctx.Context, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return envctx.Resolve("", cwd)
}

// Tools returns every tool this server exposes.
func Tools() []ToolSpec {
	return []ToolSpec{
		{
			Name:        "kdbx_list",
			Description: "List entry paths in the active vault. Never returns secret values.",
			Handler: func(_ context.Context, args map[string]any) (string, error) {
				ctx, err := currentContext()
				if err != nil {
					return "", err
				}
				h, err := vault.Open(ctx.Vault, ctx.KeyFile)
				if err != nil {
					return "", err
				}
				defer h.Close()
				entries, err := h.ListEntries()
				if err != nil {
					return "", err
				}
				group := str(args, "group")
				var b bytes.Buffer
				for _, p := range entries {
					if group == "" || strings.HasPrefix(p, group) {
						b.WriteString(p + "\n")
					}
				}
				return b.String(), nil
			},
		},
		{
			Name:        "kdbx_envs",
			Description: "List configured environments and mark the active one.",
			Handler: func(_ context.Context, _ map[string]any) (string, error) {
				ctx, err := currentContext()
				if err != nil {
					return "", err
				}
				var b bytes.Buffer
				for _, name := range ctx.Pointer.EnvNames() {
					marker := "  "
					if name == ctx.Env {
						marker = "* "
					}
					b.WriteString(marker + name + "\n")
				}
				fmt.Fprintf(&b, "active: %s (source: %s)\n", ctx.Env, ctx.Source)
				return b.String(), nil
			},
		},
		{
			Name:        "kdbx_check",
			Description: "Verify every mapped variable resolves in the active environment.",
			Handler: func(_ context.Context, _ map[string]any) (string, error) {
				ctx, err := currentContext()
				if err != nil {
					return "", err
				}
				h, err := vault.Open(ctx.Vault, ctx.KeyFile)
				if err != nil {
					return "", err
				}
				defer h.Close()
				var b bytes.Buffer
				missing := 0
				for _, name := range ctx.VarOrder {
					path := ctx.Vars[name]
					group, title, field, perr := pointer.ParseEntryPath(path)
					if perr == nil {
						if _, gerr := h.GetField(group, title, field); gerr == nil {
							continue
						}
					}
					missing++
					fmt.Fprintf(&b, "MISSING %s -> %s\n", name, path)
				}
				if missing == 0 {
					b.WriteString("ok: every mapped var resolves\n")
				}
				return b.String(), nil
			},
		},
		{
			Name: "kdbx_get",
			Description: "Confirm a secret exists at PATH. Always masked — this tool never " +
				"returns a secret value. Use kdbx_run to actually use a secret.",
			Handler: func(_ context.Context, args map[string]any) (string, error) {
				path := str(args, "path")
				group, title, field, err := pointer.ParseEntryPath(path)
				if err != nil {
					return "", err
				}
				ctx, err := currentContext()
				if err != nil {
					return "", err
				}
				h, err := vault.Open(ctx.Vault, ctx.KeyFile)
				if err != nil {
					return "", err
				}
				defer h.Close()
				if _, err := h.GetField(group, title, field); err != nil {
					return "", err
				}
				return secretio.Mask + "\n", nil
			},
		},
		{
			Name: "kdbx_run",
			Description: "Run a command with the active environment's secrets injected. The " +
				"secrets are never printed.",
			Handler: func(_ context.Context, args map[string]any) (string, error) {
				line := str(args, "command")
				argv := strings.Fields(line)
				if len(argv) == 0 {
					return "", fmt.Errorf("no command given")
				}
				ctx, err := currentContext()
				if err != nil {
					return "", err
				}
				vals, _, err := vaultvars.Resolve(ctx, false)
				if err != nil {
					return "", err
				}
				var out bytes.Buffer
				code, err := runner.Run(argv, vals, nil, &out, &out)
				if err != nil {
					return "", err
				}
				fmt.Fprintf(&out, "\n[exit %d]\n", code)
				return out.String(), nil
			},
		},
	}
}

type toolArgs struct {
	Path    string `json:"path,omitempty" jsonschema:"entry path, e.g. api/openai"`
	Group   string `json:"group,omitempty" jsonschema:"optional group prefix filter"`
	Command string `json:"command,omitempty" jsonschema:"command line to run"`
}

// Serve runs the stdio MCP server until the client disconnects.
func Serve(ctx context.Context) error {
	server := mcp.NewServer(&mcp.Implementation{Name: "kdbx", Version: "1"}, nil)
	for _, spec := range Tools() {
		mcp.AddTool(server,
			&mcp.Tool{Name: spec.Name, Description: spec.Description},
			func(ctx context.Context, _ *mcp.CallToolRequest, in toolArgs) (
				*mcp.CallToolResult, any, error) {
				out, err := spec.Handler(ctx, map[string]any{
					"path": in.Path, "group": in.Group, "command": in.Command,
				})
				if err != nil {
					return nil, nil, err
				}
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: out}},
				}, nil, nil
			})
	}
	return server.Run(ctx, &mcp.StdioTransport{})
}
