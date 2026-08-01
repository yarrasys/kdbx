package vaultvars

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yarrasys/kdbx/internal/envctx"
	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/vault"
)

func setup(t *testing.T, vars string) *envctx.Context {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("KEEPASSXC_DIR", filepath.Join(dir, "kpxc"))
	t.Setenv("KDBX_ENV", "")
	body := `{"project":"demo","defaultEnv":"dev","envs":{"dev":{"vars":` + vars + `}}}`
	if err := os.WriteFile(filepath.Join(dir, ".keepassxc.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, err := envctx.Resolve("", dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := vault.Create(ctx.Vault, ctx.KeyFile); err != nil {
		t.Fatal(err)
	}
	return ctx
}

func TestResolveReturnsValuesInPointerOrder(t *testing.T) {
	ctx := setup(t, `{"ZED":"api/zed:password","ALPHA":"api/alpha:password"}`)
	if err := vault.SetField(ctx.Vault, ctx.KeyFile, []string{"api"}, "zed", "password", "z"); err != nil {
		t.Fatal(err)
	}
	if err := vault.SetField(ctx.Vault, ctx.KeyFile, []string{"api"}, "alpha", "password", "a"); err != nil {
		t.Fatal(err)
	}
	vals, order, err := Resolve(ctx, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if vals["ZED"] != "z" || vals["ALPHA"] != "a" {
		t.Fatalf("values %v", vals)
	}
	if len(order) != 2 || order[0] != "ZED" || order[1] != "ALPHA" {
		t.Fatalf("order %v, want [ZED ALPHA]", order)
	}
}

func TestResolveUnresolvedIsDrift(t *testing.T) {
	ctx := setup(t, `{"MISSING":"api/absent:password"}`)
	if _, _, err := Resolve(ctx, false, nil); err == nil {
		t.Fatal("expected Drift for an unresolved mapping")
	}
}

func TestResolveAllowMissingSkipsUnresolved(t *testing.T) {
	ctx := setup(t, `{"PRESENT":"api/there:password","MISSING":"api/absent:password"}`)
	if err := vault.SetField(ctx.Vault, ctx.KeyFile, []string{"api"}, "there", "password", "v"); err != nil {
		t.Fatal(err)
	}
	vals, order, err := Resolve(ctx, true, nil)
	if err != nil {
		t.Fatalf("allowMissing should not fail: %v", err)
	}
	if len(vals) != 1 || vals["PRESENT"] != "v" {
		t.Fatalf("values %v", vals)
	}
	if len(order) != 1 || order[0] != "PRESENT" {
		t.Fatalf("order %v", order)
	}
}

func TestResolveVerifiesAnchor(t *testing.T) {
	ctx := setup(t, `{"API_KEY":"api/openai:password"}`)
	if err := vault.SetField(ctx.Vault, ctx.KeyFile,
		[]string{"api"}, "openai", "password", "fake-value"); err != nil {
		t.Fatal(err)
	}
	key := "kdbx:policy:dev"

	// No anchor in the vault yet: strict resolution refuses.
	if _, _, err := Resolve(ctx, false, &Anchor{Key: key, Want: "abc"}); err == nil {
		t.Fatal("missing anchor resolved without error")
	} else if kdbxerr.KindOf(err) != "PolicyDrift" {
		t.Fatalf("kind %s, want PolicyDrift", kdbxerr.KindOf(err))
	}

	if err := vault.SetCustomData(ctx.Vault, ctx.KeyFile, key, "abc"); err != nil {
		t.Fatal(err)
	}
	// Matching anchor: values resolve.
	if _, _, err := Resolve(ctx, false, &Anchor{Key: key, Want: "abc"}); err != nil {
		t.Fatalf("matching anchor refused: %v", err)
	}
	// Mismatched anchor: refused, and no values escape.
	vals, _, err := Resolve(ctx, false, &Anchor{Key: key, Want: "other"})
	if err == nil || kdbxerr.KindOf(err) != "PolicyDrift" {
		t.Fatalf("mismatch: vals=%v err=%v", vals, err)
	}
	if vals != nil {
		t.Fatal("values returned alongside a PolicyDrift refusal")
	}
}
