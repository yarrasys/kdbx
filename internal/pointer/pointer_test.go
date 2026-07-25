package pointer

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/yarrasys/kdbx/internal/paths"
)

func writePointer(t *testing.T, dir, body string) string {
	t.Helper()
	p := filepath.Join(dir, Name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const fixture = `{
  "project": "ideas",
  "defaultEnv": "dev",
  "envs": {
    "dev": {
      "vault": "${KEEPASSXC_DIR}/ideas/dev.kdbx",
      "keyFile": "${KEEPASSXC_DIR}/ideas/dev.keyx",
      "vars": {
        "OPENAI_API_KEY": "api/openai:password"
      }
    },
    "prod": {}
  }
}
`

func TestFindWalksUpFromNestedDir(t *testing.T) {
	root := t.TempDir()
	writePointer(t, root, fixture)
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Find(nested)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	wantSuffix := filepath.Join(filepath.Base(root), Name)
	if !strings.HasSuffix(got, wantSuffix) {
		t.Fatalf("found %q, want a path ending in %q", got, wantSuffix)
	}
}

func TestFindReturnsNotFoundExitCode2(t *testing.T) {
	dir := t.TempDir()
	_, err := Find(dir)
	if err == nil {
		t.Fatal("expected an error when no pointer exists")
	}
	if !strings.Contains(err.Error(), "no .keepassxc.json found") {
		t.Fatalf("message %q lacks the documented wording", err.Error())
	}
}

func TestSelectEnvPrecedence(t *testing.T) {
	dir := t.TempDir()
	p, err := Load(writePointer(t, dir, fixture))
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("KDBX_ENV", "")
	if name, src := p.SelectEnv(""); name != "dev" || src != "pointer" {
		t.Fatalf("default: got %q/%q, want dev/pointer", name, src)
	}
	t.Setenv("KDBX_ENV", "prod")
	if name, src := p.SelectEnv(""); name != "prod" || src != "$KDBX_ENV" {
		t.Fatalf("env var: got %q/%q, want prod/$KDBX_ENV", name, src)
	}
	if name, src := p.SelectEnv("staging"); name != "staging" || src != "--env" {
		t.Fatalf("flag: got %q/%q, want staging/--env", name, src)
	}
}

func TestResolveEnvExpandsTokensAndReadsVars(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "kpxc")
	t.Setenv("KEEPASSXC_DIR", base)
	p, _ := Load(writePointer(t, dir, fixture))

	ep, err := p.ResolveEnv("dev")
	if err != nil {
		t.Fatalf("ResolveEnv: %v", err)
	}
	if want := filepath.Join(paths.Resolve(base), "ideas", "dev.kdbx"); ep.Vault != want {
		t.Fatalf("vault %q, want %q", ep.Vault, want)
	}
	if want := filepath.Join(paths.Resolve(base), "ideas", "dev.keyx"); ep.KeyFile != want {
		t.Fatalf("keyfile %q, want %q", ep.KeyFile, want)
	}
	if ep.Vars["OPENAI_API_KEY"] != "api/openai:password" {
		t.Fatalf("vars %v", ep.Vars)
	}
}

func TestResolveEnvDefaultsPathsFromProject(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "kpxc")
	t.Setenv("KEEPASSXC_DIR", base)
	p, _ := Load(writePointer(t, dir, `{"project":"ideas","defaultEnv":"dev","envs":{"dev":{}}}`))

	ep, err := p.ResolveEnv("dev")
	if err != nil {
		t.Fatalf("ResolveEnv: %v", err)
	}
	if want := filepath.Join(paths.Resolve(base), "ideas", "dev.kdbx"); ep.Vault != want {
		t.Fatalf("vault %q, want %q", ep.Vault, want)
	}
}

func TestResolveEnvProjectFallsBackToPointerDirName(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "myrepo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(dir, "kpxc")
	t.Setenv("KEEPASSXC_DIR", base)
	p, _ := Load(writePointer(t, dir, `{"defaultEnv":"dev","envs":{"dev":{}}}`))

	ep, _ := p.ResolveEnv("dev")
	if want := filepath.Join(paths.Resolve(base), "myrepo", "dev.kdbx"); ep.Vault != want {
		t.Fatalf("vault %q, want %q", ep.Vault, want)
	}
}

func TestResolveEnvUnknownEnvIsNotFound(t *testing.T) {
	dir := t.TempDir()
	p, _ := Load(writePointer(t, dir, fixture))
	if _, err := p.ResolveEnv("nope"); err == nil {
		t.Fatal("expected an error for an unconfigured env")
	}
}

func TestSetVarAndSavePreservesFormatting(t *testing.T) {
	dir := t.TempDir()
	path := writePointer(t, dir, fixture)
	p, _ := Load(path)
	p.SetVar("dev", "NEW_KEY", "api/new:password")
	if err := p.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, _ := os.ReadFile(path)
	s := string(got)
	if !strings.HasPrefix(s, "{\n  \"project\": \"ideas\",") {
		t.Fatalf("top-level key order or indentation changed:\n%s", s)
	}
	if !strings.HasSuffix(s, "}\n") {
		t.Fatalf("missing trailing newline:\n%q", s)
	}
	if !strings.Contains(s, `"NEW_KEY": "api/new:password"`) {
		t.Fatalf("var not written:\n%s", s)
	}
}

func TestSetVarCreatesMissingEnvAndVarsLevels(t *testing.T) {
	dir := t.TempDir()
	path := writePointer(t, dir, fixture)
	p, _ := Load(path)
	p.SetVar("prod", "PROD_KEY", "api/prod:password")
	if err := p.Save(); err != nil {
		t.Fatal(err)
	}
	reloaded, _ := Load(path)
	ep, err := reloaded.ResolveEnv("prod")
	if err != nil {
		t.Fatal(err)
	}
	if ep.Vars["PROD_KEY"] != "api/prod:password" {
		t.Fatalf("vars %v", ep.Vars)
	}
}

func TestRepointVarsPreservesFieldSuffix(t *testing.T) {
	dir := t.TempDir()
	path := writePointer(t, dir, `{
  "defaultEnv": "dev",
  "envs": {
    "dev": {
      "vars": {
        "A": "old/entry:password",
        "B": "old/entry:CUSTOM",
        "C": "other/entry:password"
      }
    }
  }
}
`)
	p, _ := Load(path)
	n := p.RepointVars("dev", "old/entry", "new/place")
	if n != 2 {
		t.Fatalf("repointed %d, want 2", n)
	}
	if err := p.Save(); err != nil {
		t.Fatal(err)
	}
	reloaded, _ := Load(path)
	ep, _ := reloaded.ResolveEnv("dev")
	if ep.Vars["A"] != "new/place:password" {
		t.Fatalf("A = %q", ep.Vars["A"])
	}
	if ep.Vars["B"] != "new/place:CUSTOM" {
		t.Fatalf("B = %q (field suffix must be preserved)", ep.Vars["B"])
	}
	if ep.Vars["C"] != "other/entry:password" {
		t.Fatalf("C = %q (unrelated mapping must not change)", ep.Vars["C"])
	}
}

func TestEnvNamesInFileOrder(t *testing.T) {
	dir := t.TempDir()
	p, _ := Load(writePointer(t, dir, fixture))
	got := p.EnvNames()
	if len(got) != 2 || got[0] != "dev" || got[1] != "prod" {
		t.Fatalf("EnvNames = %v, want [dev prod]", got)
	}
}

func TestFindResolvesSymlinksSoProjectNameMatchesPython(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs elevation on Windows")
	}
	root := t.TempDir()
	realDir := filepath.Join(root, "myrepo")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writePointer(t, realDir, `{"defaultEnv":"dev","envs":{"dev":{}}}`)

	link := filepath.Join(root, "aliased-name")
	if err := os.Symlink(realDir, link); err != nil {
		t.Fatal(err)
	}

	viaReal, err := Find(realDir)
	if err != nil {
		t.Fatal(err)
	}
	viaLink, err := Find(link)
	if err != nil {
		t.Fatal(err)
	}
	if viaReal != viaLink {
		t.Fatalf("Find via symlink returned %q, want %q", viaLink, viaReal)
	}

	// The project name (and therefore the default vault path) must not depend
	// on which name the user used to reach the repo.
	pReal, _ := Load(viaReal)
	pLink, _ := Load(viaLink)
	if pReal.Project() != pLink.Project() {
		t.Fatalf("project name differs by access path: %q vs %q", pReal.Project(), pLink.Project())
	}
	if got := pLink.Project(); got != "myrepo" {
		t.Fatalf("project = %q, want the real directory name %q", got, "myrepo")
	}
}
