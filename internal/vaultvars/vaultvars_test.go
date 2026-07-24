package vaultvars

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yarrasys/kdbx/internal/envctx"
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
	vals, order, err := Resolve(ctx, false)
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
	if _, _, err := Resolve(ctx, false); err == nil {
		t.Fatal("expected Drift for an unresolved mapping")
	}
}

func TestResolveAllowMissingSkipsUnresolved(t *testing.T) {
	ctx := setup(t, `{"PRESENT":"api/there:password","MISSING":"api/absent:password"}`)
	if err := vault.SetField(ctx.Vault, ctx.KeyFile, []string{"api"}, "there", "password", "v"); err != nil {
		t.Fatal(err)
	}
	vals, order, err := Resolve(ctx, true)
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
