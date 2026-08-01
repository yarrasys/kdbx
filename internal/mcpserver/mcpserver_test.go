package mcpserver

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/yarrasys/kdbx/internal/envctx"
	"github.com/yarrasys/kdbx/internal/vault"
)

func project(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("KEEPASSXC_DIR", filepath.Join(dir, "kpxc"))
	t.Setenv("KDBX_ENV", "")
	body := `{"project":"demo","defaultEnv":"dev","envs":{"dev":{"vars":{"API_KEY":"api/openai:password"}},"prod":{}}}`
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
	if err := vault.SetField(ctx.Vault, ctx.KeyFile, []string{"api"}, "openai", "password", "sk-secret"); err != nil {
		t.Fatal(err)
	}
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return dir
}

func handler(t *testing.T, name string) func(context.Context, map[string]any) (string, error) {
	t.Helper()
	for _, spec := range Tools() {
		if spec.Name == name {
			return spec.Handler
		}
	}
	t.Fatalf("tool %q not registered", name)
	return nil
}

func TestExposesExactlyTheFiveReadOnlyTools(t *testing.T) {
	got := map[string]bool{}
	for _, spec := range Tools() {
		got[spec.Name] = true
		if spec.Description == "" {
			t.Errorf("tool %q has no description", spec.Name)
		}
	}
	want := []string{"kdbx_list", "kdbx_envs", "kdbx_check", "kdbx_get", "kdbx_run"}
	if len(got) != len(want) {
		t.Fatalf("registered %v, want exactly %v", got, want)
	}
	for _, n := range want {
		if !got[n] {
			t.Errorf("missing tool %q", n)
		}
	}
	for _, forbidden := range []string{"kdbx_set", "kdbx_delete", "kdbx_export", "kdbx_import", "kdbx_rekey", "kdbx_mv"} {
		if got[forbidden] {
			t.Errorf("write tool %q must never be exposed over MCP", forbidden)
		}
	}
}

func TestGetToolNeverReturnsAValue(t *testing.T) {
	project(t)
	out, err := handler(t, "kdbx_get")(context.Background(), map[string]any{"path": "api/openai"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "sk-secret") {
		t.Fatalf("kdbx_get leaked the value: %q", out)
	}
	if !strings.Contains(out, "(set, hidden)") {
		t.Fatalf("expected the mask, got %q", out)
	}
}

func TestListToolReturnsPaths(t *testing.T) {
	project(t)
	out, err := handler(t, "kdbx_list")(context.Background(), map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "api/openai") {
		t.Fatalf("list output %q", out)
	}
	if strings.Contains(out, "sk-secret") {
		t.Fatal("list leaked a value")
	}
}

func TestEnvsToolMarksTheActiveEnv(t *testing.T) {
	project(t)
	out, err := handler(t, "kdbx_envs")(context.Background(), map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "dev") || !strings.Contains(out, "prod") {
		t.Fatalf("envs output %q", out)
	}
}

func TestCheckToolReportsResolution(t *testing.T) {
	project(t)
	out, err := handler(t, "kdbx_check")(context.Background(), map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "MISSING") {
		t.Fatalf("expected a clean check, got %q", out)
	}
}

// kdbx_run is the one MCP tool that handles real secret values, so it gets the
// strictest test: the child must receive the secret, and the secret must not
// appear in what the tool hands back to the model.
func TestRunToolInjectsTheSecretButDoesNotReturnIt(t *testing.T) {
	project(t)

	out, err := handler(t, "kdbx_run")(context.Background(), map[string]any{
		"command": "sh -c " + strconv.Quote("test \"$API_KEY\" = sk-secret && echo INJECTED_OK || echo INJECTED_MISSING"),
	})
	if err != nil {
		t.Fatalf("kdbx_run: %v", err)
	}
	if !strings.Contains(out, "INJECTED_OK") {
		t.Fatalf("child did not receive the injected secret; output was:\n%s", out)
	}
	if strings.Contains(out, "sk-secret") {
		t.Fatalf("kdbx_run leaked the secret value into its result:\n%s", out)
	}
	if !strings.Contains(out, "[exit 0]") {
		t.Fatalf("expected the exit-code footer, got:\n%s", out)
	}
}

func TestRunToolReportsAChildFailureExitCode(t *testing.T) {
	project(t)

	out, err := handler(t, "kdbx_run")(context.Background(), map[string]any{
		"command": "sh -c " + strconv.Quote("exit 3"),
	})
	if err != nil {
		t.Fatalf("kdbx_run should report a failing child via its footer, not an error: %v", err)
	}
	if !strings.Contains(out, "[exit 3]") {
		t.Fatalf("expected [exit 3] footer, got:\n%s", out)
	}
}

func TestRunToolRejectsAnEmptyCommand(t *testing.T) {
	project(t)
	if _, err := handler(t, "kdbx_run")(context.Background(), map[string]any{"command": "   "}); err == nil {
		t.Fatal("expected an error for an empty command")
	}
}

func TestRunToolMasksAnEchoedSecret(t *testing.T) {
	project(t)
	out, err := handler(t, "kdbx_run")(context.Background(), map[string]any{
		"command": "sh -c " + strconv.Quote("echo $API_KEY"),
	})
	if err != nil {
		t.Fatalf("kdbx_run: %v", err)
	}
	if strings.Contains(out, "sk-secret") {
		t.Fatalf("kdbx_run returned the raw value to the model:\n%s", out)
	}
	if !strings.Contains(out, "***") {
		t.Fatalf("expected the mask in the output, got:\n%s", out)
	}
}
