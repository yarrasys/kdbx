package audit

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAppendWritesOneLinePerCall(t *testing.T) {
	p := filepath.Join(t.TempDir(), "dev.kdbx.audit.log")
	if err := Append(p, "run", []string{"npm", "test"}, []string{"OPENAI_API_KEY"}); err != nil {
		t.Fatal(err)
	}
	if err := Append(p, "refused", []string{"env"}, nil); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d: %q", len(lines), string(b))
	}
	if !strings.Contains(lines[0], "\trun\t") ||
		!strings.Contains(lines[0], "npm test") ||
		!strings.Contains(lines[0], "OPENAI_API_KEY") {
		t.Fatalf("line 0 = %q", lines[0])
	}
	if !strings.Contains(lines[1], "\trefused\t") || !strings.Contains(lines[1], "env") {
		t.Fatalf("line 1 = %q", lines[1])
	}
}

func TestAppendCreatesFilePrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits")
	}
	p := filepath.Join(t.TempDir(), "a.log")
	if err := Append(p, "run", []string{"x"}, nil); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("perm %v, want 0600", st.Mode().Perm())
	}
}

func TestAppendNeverWritesValues(t *testing.T) {
	// The API cannot even express a value: it takes var *names*. This test
	// pins the argv/name-only shape so a future "helpful" refactor that adds
	// values shows up as a diff here.
	p := filepath.Join(t.TempDir(), "a.log")
	if err := Append(p, "run", []string{"printenv"}, []string{"KEY_NAME"}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if !strings.Contains(string(b), "KEY_NAME") {
		t.Fatalf("var name missing: %q", b)
	}
}
