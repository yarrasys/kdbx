// Package vaultvars resolves an environment's mapped variables to their secret
// values. It is the single implementation shared by `run` and `export`.
package vaultvars

import (
	"github.com/yarrasys/kdbx/internal/envctx"
	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/pointer"
	"github.com/yarrasys/kdbx/internal/vault"
)

// Resolve reads every mapped var. Unless allowMissing is set, the first var
// that does not resolve is a Drift error (exit 5).
func Resolve(ctx *envctx.Context, allowMissing bool) (map[string]string, []string, error) {
	h, err := vault.Open(ctx.Vault, ctx.KeyFile)
	if err != nil {
		return nil, nil, err
	}
	defer h.Close()

	vals := map[string]string{}
	var order []string
	for _, name := range ctx.VarOrder {
		path := ctx.Vars[name]
		group, title, field, perr := pointer.ParseEntryPath(path)
		if perr != nil {
			if allowMissing {
				continue
			}
			return nil, nil, kdbxerr.Drift("unresolved var %s -> %s", name, path)
		}
		v, gerr := h.GetField(group, title, field)
		if gerr != nil {
			if allowMissing {
				continue
			}
			return nil, nil, kdbxerr.Drift("unresolved var %s -> %s", name, path)
		}
		vals[name] = v
		order = append(order, name)
	}
	return vals, order, nil
}
