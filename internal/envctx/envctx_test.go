package envctx

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupProject(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".keepassxc.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestResolveUsesPointerDefault(t *testing.T) {
	t.Setenv("KDBX_ENV", "")
	t.Setenv("KEEPASSXC_DIR", t.TempDir())
	dir := setupProject(t, `{"project":"p","defaultEnv":"dev","envs":{"dev":{"vars":{"K":"api/k:password"}}}}`)
	c, err := Resolve("", dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if c.Env != "dev" || c.Source != "pointer" {
		t.Fatalf("env=%q source=%q", c.Env, c.Source)
	}
	if c.Vars["K"] != "api/k:password" {
		t.Fatalf("vars %v", c.Vars)
	}
}

func TestResolveNoPointerIsNotFound(t *testing.T) {
	if _, err := Resolve("", t.TempDir()); err == nil {
		t.Fatal("expected NotFound when no pointer exists")
	}
}

func TestBannerFormatMatchesPython(t *testing.T) {
	t.Setenv("KDBX_ENV", "")
	t.Setenv("KEEPASSXC_DIR", t.TempDir())
	dir := setupProject(t, `{"project":"p","defaultEnv":"dev","envs":{"dev":{}}}`)
	c, _ := Resolve("", dir)
	var buf bytes.Buffer
	c.WriteBanner(&buf)
	got := buf.String()
	if !strings.HasPrefix(got, "ACTIVE ENV: dev  vault=") {
		t.Fatalf("banner prefix wrong: %q", got)
	}
	if !strings.HasSuffix(got, "(source: pointer)\n") {
		t.Fatalf("banner suffix wrong: %q", got)
	}
}
