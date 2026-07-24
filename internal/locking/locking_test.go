package locking

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCaptureStateOfMissingFileIsEmpty(t *testing.T) {
	got, err := CaptureState(filepath.Join(t.TempDir(), "absent.kdbx"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestVerifyUnchangedDetectsModification(t *testing.T) {
	p := filepath.Join(t.TempDir(), "v.kdbx")
	if err := os.WriteFile(p, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	captured, err := CaptureState(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyUnchanged(p, captured); err != nil {
		t.Fatalf("unchanged file should verify: %v", err)
	}
	if err := os.WriteFile(p, []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyUnchanged(p, captured); err == nil {
		t.Fatal("modified file must fail verification")
	}
}

func TestWithVaultLockRunsAndReleases(t *testing.T) {
	p := filepath.Join(t.TempDir(), "v.kdbx")
	ran := false
	if err := WithVaultLock(p, func() error { ran = true; return nil }); err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Fatal("callback did not run")
	}
	// The lock must be released, so a second acquisition succeeds immediately.
	if err := WithVaultLock(p, func() error { return nil }); err != nil {
		t.Fatalf("lock was not released: %v", err)
	}
}

func TestWithVaultLockPropagatesCallbackError(t *testing.T) {
	p := filepath.Join(t.TempDir(), "v.kdbx")
	sentinel := os.ErrClosed
	if err := WithVaultLock(p, func() error { return sentinel }); err == nil {
		t.Fatal("callback error must propagate")
	}
}
