package secretio

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReadSecretFromEnv(t *testing.T) {
	t.Setenv("MY_SECRET_SRC", "s3cr3t")
	got, err := ReadSecret(ReadOpts{FromEnv: "MY_SECRET_SRC"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "s3cr3t" {
		t.Fatalf("got %q", got)
	}
}

func TestReadSecretFromEnvUnsetIsAnError(t *testing.T) {
	os.Unsetenv("DEFINITELY_UNSET_VAR")
	if _, err := ReadSecret(ReadOpts{FromEnv: "DEFINITELY_UNSET_VAR"}); err == nil {
		t.Fatal("expected an error when --from-env names an unset variable")
	}
}

func TestReadSecretFromStdinStripsOneTrailingNewline(t *testing.T) {
	got, err := ReadSecret(ReadOpts{Stdin: strings.NewReader("value\n")})
	if err != nil {
		t.Fatal(err)
	}
	if got != "value" {
		t.Fatalf("got %q, want %q", got, "value")
	}
}

func TestReadSecretStripsCRLF(t *testing.T) {
	got, err := ReadSecret(ReadOpts{Stdin: strings.NewReader("value\r\n")})
	if err != nil {
		t.Fatal(err)
	}
	if got != "value" {
		t.Fatalf("got %q", got)
	}
}

func TestReadSecretRawKeepsTrailingNewline(t *testing.T) {
	got, err := ReadSecret(ReadOpts{Stdin: strings.NewReader("value\n"), Raw: true})
	if err != nil {
		t.Fatal(err)
	}
	if got != "value\n" {
		t.Fatalf("got %q", got)
	}
}

func TestReadSecretEmptyStdinIsAnError(t *testing.T) {
	if _, err := ReadSecret(ReadOpts{Stdin: strings.NewReader("   \n")}); err == nil {
		t.Fatal("expected an error for whitespace-only stdin")
	}
}

func TestReadSecretPromptsAndConfirmsOnTTY(t *testing.T) {
	calls := 0
	got, err := ReadSecret(ReadOpts{
		IsTTY: true,
		PromptFn: func(prompt string) (string, error) {
			calls++
			return "typed", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "typed" {
		t.Fatalf("got %q", got)
	}
	if calls != 2 {
		t.Fatalf("prompt called %d times, want 2 (value + confirm)", calls)
	}
}

func TestReadSecretMismatchedConfirmationFails(t *testing.T) {
	seq := []string{"one", "two"}
	i := 0
	_, err := ReadSecret(ReadOpts{
		IsTTY: true,
		PromptFn: func(string) (string, error) {
			v := seq[i]
			i++
			return v, nil
		},
	})
	if err == nil {
		t.Fatal("expected an error when the confirmation does not match")
	}
}

func TestConfirmRefusesWithoutTTY(t *testing.T) {
	var errBuf bytes.Buffer
	if Confirm("purge it?", strings.NewReader("y\n"), &errBuf, false) {
		t.Fatal("must refuse without a TTY")
	}
	if !strings.Contains(errBuf.String(), "needs an interactive terminal to confirm") {
		t.Fatalf("stderr %q lacks the documented wording", errBuf.String())
	}
}

func TestConfirmAcceptsYOnTTY(t *testing.T) {
	var errBuf bytes.Buffer
	if !Confirm("go?", strings.NewReader("y\n"), &errBuf, true) {
		t.Fatal("y should confirm")
	}
	if Confirm("go?", strings.NewReader("n\n"), &errBuf, true) {
		t.Fatal("n should refuse")
	}
	if Confirm("go?", strings.NewReader("\n"), &errBuf, true) {
		t.Fatal("empty should refuse (default N)")
	}
}

func TestAtomicWriteSecretIsOwnerOnly(t *testing.T) {
	p := filepath.Join(t.TempDir(), "secret.txt")
	if err := AtomicWriteSecret(p, []byte("data")); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil || string(b) != "data" {
		t.Fatalf("content %q err %v", b, err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if perm := st.Mode().Perm(); perm != 0o600 {
		t.Fatalf("mode %o, want 600", perm)
	}
}
