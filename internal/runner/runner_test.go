package runner

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// buildEcho compiles a tiny helper that prints an env var and exits with a code.
func buildEcho(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "helper.go")
	code := `package main

import (
	"fmt"
	"os"
	"strconv"
)

func main() {
	fmt.Println(os.Getenv("KDBX_TEST_VAR"))
	if len(os.Args) > 1 {
		n, _ := strconv.Atoi(os.Args[1])
		os.Exit(n)
	}
}
`
	if err := os.WriteFile(src, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "helper")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, src)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building helper: %v\n%s", err, out)
	}
	return bin
}

func TestRunInjectsVariables(t *testing.T) {
	bin := buildEcho(t)
	var out bytes.Buffer
	code, err := Run([]string{bin}, map[string]string{"KDBX_TEST_VAR": "injected"}, nil, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("exit code %d, want 0", code)
	}
	if strings.TrimSpace(out.String()) != "injected" {
		t.Fatalf("child saw %q", out.String())
	}
}

func TestRunPassesThroughExitCode(t *testing.T) {
	bin := buildEcho(t)
	var out bytes.Buffer
	code, err := Run([]string{bin, "42"}, nil, nil, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	if code != 42 {
		t.Fatalf("exit code %d, want 42", code)
	}
}

func TestRunInheritsParentEnvironment(t *testing.T) {
	t.Setenv("KDBX_TEST_VAR", "from-parent")
	bin := buildEcho(t)
	var out bytes.Buffer
	if _, err := Run([]string{bin}, nil, nil, &out, &out); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != "from-parent" {
		t.Fatalf("child saw %q, want the inherited value", out.String())
	}
}

func TestRunInjectionOverridesParentEnvironment(t *testing.T) {
	t.Setenv("KDBX_TEST_VAR", "from-parent")
	bin := buildEcho(t)
	var out bytes.Buffer
	if _, err := Run([]string{bin}, map[string]string{"KDBX_TEST_VAR": "override"}, nil, &out, &out); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != "override" {
		t.Fatalf("child saw %q, want the injected value", out.String())
	}
}

func TestLookupResolvesViaPath(t *testing.T) {
	bin := buildEcho(t)
	t.Setenv("PATH", filepath.Dir(bin)+string(os.PathListSeparator)+os.Getenv("PATH"))
	got, err := Lookup("helper")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if filepath.Base(got) != filepath.Base(bin) {
		t.Fatalf("resolved %q, want %q", got, bin)
	}
}

func TestLookupUnknownCommandIsNotFound(t *testing.T) {
	if _, err := Lookup("definitely-not-a-real-binary-xyz"); err == nil {
		t.Fatal("expected NotFound for an unresolvable command")
	}
}

func TestRunUnresolvableCommandIsNotFound(t *testing.T) {
	if _, err := Run([]string{"definitely-not-a-real-binary-xyz"}, nil, nil, nil, nil); err == nil {
		t.Fatal("expected an error for an unresolvable command")
	}
}
