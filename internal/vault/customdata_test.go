package vault

import (
	"path/filepath"
	"testing"
)

func TestCustomDataRoundTrips(t *testing.T) {
	dir := t.TempDir()
	vp, kp := filepath.Join(dir, "dev.kdbx"), filepath.Join(dir, "dev.keyx")
	if err := Create(vp, kp); err != nil {
		t.Fatal(err)
	}

	if err := SetCustomData(vp, kp, "kdbx:policy:dev", "abc123"); err != nil {
		t.Fatalf("SetCustomData: %v", err)
	}

	h, err := Open(vp, kp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = h.Close() }()
	if got := h.CustomData("kdbx:policy:dev"); got != "abc123" {
		t.Fatalf("CustomData = %q, want abc123", got)
	}
	if got := h.CustomData("nope"); got != "" {
		t.Fatalf("unknown key = %q, want empty", got)
	}
}

func TestSetCustomDataReplacesExistingKey(t *testing.T) {
	dir := t.TempDir()
	vp, kp := filepath.Join(dir, "dev.kdbx"), filepath.Join(dir, "dev.keyx")
	if err := Create(vp, kp); err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"first", "second"} {
		if err := SetCustomData(vp, kp, "k", v); err != nil {
			t.Fatal(err)
		}
	}
	h, err := Open(vp, kp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = h.Close() }()
	if got := h.CustomData("k"); got != "second" {
		t.Fatalf("CustomData = %q, want second (replaced, not appended)", got)
	}
}
