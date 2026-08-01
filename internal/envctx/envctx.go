// Package envctx resolves the active environment for a command invocation and
// prints the ACTIVE ENV banner (spec C1, C5).
package envctx

import (
	"fmt"
	"io"

	"github.com/yarrasys/kdbx/internal/pointer"
)

// Context is the resolved environment for one command invocation.
type Context struct {
	Env      string
	Source   string
	Vault    string
	KeyFile  string
	Vars     map[string]string
	VarOrder []string
	Allow    []string
	AllowSet bool
	Pointer  *pointer.Pointer
}

// Resolve finds the pointer from startDir, selects the environment, and
// resolves its artifact paths and var mappings.
func Resolve(cliEnv, startDir string) (*Context, error) {
	path, err := pointer.Find(startDir)
	if err != nil {
		return nil, err
	}
	p, err := pointer.Load(path)
	if err != nil {
		return nil, err
	}
	env, source := p.SelectEnv(cliEnv)
	ep, err := p.ResolveEnv(env)
	if err != nil {
		return nil, err
	}
	return &Context{
		Env: env, Source: source,
		Vault: ep.Vault, KeyFile: ep.KeyFile,
		Vars: ep.Vars, VarOrder: ep.VarOrder,
		Allow: ep.Allow, AllowSet: ep.AllowSet,
		Pointer: p,
	}, nil
}

// WriteBanner tells the operator which vault is about to be touched.
func (c *Context) WriteBanner(w io.Writer) {
	fmt.Fprintf(w, "ACTIVE ENV: %s  vault=%s  (source: %s)\n", c.Env, c.Vault, c.Source)
}
