// Package runpolicy is the single gate every injection path goes through —
// the CLI's `run` and the MCP server's `kdbx_run` — so the allowlist, the
// strict-mode rules and the audit trail cannot drift between surfaces
// (spec C5, N6).
package runpolicy

import (
	"github.com/yarrasys/kdbx/internal/allowlist"
	"github.com/yarrasys/kdbx/internal/audit"
	"github.com/yarrasys/kdbx/internal/envctx"
	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/pointer"
	"github.com/yarrasys/kdbx/internal/vaultvars"
)

// Flags are the caller-requested escape hatches. The MCP server always passes
// the zero value: an agent surface has no business bypassing anything.
type Flags struct {
	NoMask bool
	Any    bool
}

// Strict reports whether ctx's env runs under the strict policy.
func Strict(ctx *envctx.Context) bool { return ctx.Mode == "strict" }

// AuditPath is where env's audit lines go: next to the vault, outside the
// repo, one file per env.
func AuditPath(ctx *envctx.Context) string { return ctx.Vault + ".audit.log" }

// Gate applies the env's run policy to argv, before the vault is opened. It
// returns the policy anchor to verify at vault-open time (nil unless strict).
// A refusal under strict is audited before it is returned; if the audit line
// cannot be written the audit error wins, so a full disk cannot silence the
// trail.
func Gate(ctx *envctx.Context, argv []string, f Flags) (*vaultvars.Anchor, error) {
	strict := Strict(ctx)
	refuse := func(e *kdbxerr.Error) error {
		if strict {
			if aerr := audit.Append(AuditPath(ctx), "refused", argv, nil); aerr != nil {
				return kdbxerr.Wrap(aerr, "Runtime", 1, "writing audit log")
			}
		}
		return e
	}

	if strict {
		if f.NoMask {
			return nil, refuse(kdbxerr.NotAllowed("kdbx run: --no-mask is refused by this env's strict policy"))
		}
		if f.Any {
			return nil, refuse(kdbxerr.NotAllowed("kdbx run: --any is refused by this env's strict policy"))
		}
		if !ctx.AllowSet {
			return nil, refuse(kdbxerr.NotAllowed(
				"kdbx run: strict policy requires a run.allow list in .keepassxc.json"))
		}
	}

	if ctx.AllowSet && !f.Any {
		ok, err := allowlist.Match(ctx.Allow, argv)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, refuse(kdbxerr.NotAllowed(
				"kdbx run: command not in this env's run.allow list " +
					"(add it to .keepassxc.json, or a human can pass --any)"))
		}
	}

	if strict {
		want, err := ctx.Pointer.PolicyHash(ctx.Env)
		if err != nil {
			return nil, err
		}
		return &vaultvars.Anchor{Key: pointer.PolicyAnchorKey(ctx.Env), Want: want}, nil
	}
	return nil, nil
}

// Record appends the post-resolution audit line under strict: the command
// that ran and the names (never the values) of what was injected.
func Record(ctx *envctx.Context, argv, varNames []string) error {
	if !Strict(ctx) {
		return nil
	}
	if err := audit.Append(AuditPath(ctx), "run", argv, varNames); err != nil {
		return kdbxerr.Wrap(err, "Runtime", 1, "writing audit log")
	}
	return nil
}
