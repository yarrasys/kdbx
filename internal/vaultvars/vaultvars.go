// Package vaultvars resolves an environment's mapped variables to their secret
// values. It is the single implementation shared by `run` and `export`.
package vaultvars

import (
	"github.com/yarrasys/kdbx/internal/envctx"
	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/pointer"
	"github.com/yarrasys/kdbx/internal/vault"
)

// Anchor is a policy expectation to verify against the vault's custom data
// while it is open: the value stored under Key must equal Want.
type Anchor struct {
	Key  string
	Want string
}

// Resolve reads every mapped var. Unless allowMissing is set, the first var
// that does not resolve is a Drift error (exit 5). A non-nil anchor is
// verified first, on the same vault open — under a strict policy no value
// leaves the vault until the pointer's policy matches what a human blessed.
func Resolve(ctx *envctx.Context, allowMissing bool, anchor *Anchor) (map[string]string, []string, error) {
	h, err := vault.Open(ctx.Vault, ctx.KeyFile)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = h.Close() }()

	if anchor != nil {
		switch got := h.CustomData(anchor.Key); got {
		case anchor.Want:
		case "":
			return nil, nil, kdbxerr.PolicyDrift(
				"strict policy is not blessed for env '%s' (a human runs: kdbx policy bless)", ctx.Env)
		default:
			return nil, nil, kdbxerr.PolicyDrift(
				"policy changed since it was blessed for env '%s' (review .keepassxc.json, then a human runs: kdbx policy bless)", ctx.Env)
		}
	}

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
