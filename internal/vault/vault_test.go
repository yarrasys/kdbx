package vault

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestCreateProducesAnOpenableKdbx4Vault(t *testing.T) {
	v, k := newVault(t)
	if _, err := os.Stat(v); err != nil {
		t.Fatalf("vault not created: %v", err)
	}
	h, err := Open(v, k)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer h.Close()
	if runtime.GOOS != "windows" {
		st, _ := os.Stat(v)
		if perm := st.Mode().Perm(); perm != 0o600 {
			t.Fatalf("vault mode %o, want 600", perm)
		}
	}
}

func TestCreateRefusesToOverwrite(t *testing.T) {
	v, k := newVault(t)
	if err := Create(v, k); err == nil {
		t.Fatal("Create must refuse when the vault already exists")
	}
}

func TestOpenWithMissingKeyfileIsLocked(t *testing.T) {
	v, _ := newVault(t)
	_, err := Open(v, filepath.Join(t.TempDir(), "absent.keyx"))
	if err == nil {
		t.Fatal("expected an error opening with a missing keyfile")
	}
}

func TestSetGetReservedAndCustomFields(t *testing.T) {
	t.Skip("write path lands in Task 12")
	v, k := newVault(t)
	if err := SetField(v, k, []string{"api"}, "openai", "password", "sk-test"); err != nil {
		t.Fatalf("SetField password: %v", err)
	}
	if err := SetField(v, k, []string{"api"}, "openai", "ORG_ID", "org-123"); err != nil {
		t.Fatalf("SetField custom: %v", err)
	}

	h, err := Open(v, k)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()

	if got, err := h.GetField([]string{"api"}, "openai", "password"); err != nil || got != "sk-test" {
		t.Fatalf("password = %q, err %v", got, err)
	}
	if got, err := h.GetField([]string{"api"}, "openai", "ORG_ID"); err != nil || got != "org-123" {
		t.Fatalf("custom = %q, err %v", got, err)
	}
	if got, err := h.GetField([]string{"api"}, "openai", "PASSWORD"); err != nil || got != "sk-test" {
		t.Fatalf("reserved field lookup must be case-insensitive: %q %v", got, err)
	}
}

func TestGetFieldMissingEntryIsNotFound(t *testing.T) {
	v, k := newVault(t)
	h, _ := Open(v, k)
	defer h.Close()
	if _, err := h.GetField([]string{"api"}, "nope", "password"); err == nil {
		t.Fatal("expected NotFound for a missing entry")
	}
}

func TestGetFieldMissingFieldIsNotFound(t *testing.T) {
	t.Skip("write path lands in Task 12")
	v, k := newVault(t)
	if err := SetField(v, k, []string{"api"}, "openai", "password", "sk-test"); err != nil {
		t.Fatal(err)
	}
	h, _ := Open(v, k)
	defer h.Close()
	if _, err := h.GetField([]string{"api"}, "openai", "ABSENT"); err == nil {
		t.Fatal("expected NotFound for an absent custom field")
	}
}

func TestSetFieldRefusesEmptyValue(t *testing.T) {
	t.Skip("write path lands in Task 12")
	v, k := newVault(t)
	if err := SetField(v, k, []string{"api"}, "x", "password", "   "); err == nil {
		t.Fatal("SetField must refuse a whitespace-only value")
	}
}

func TestListEntriesIsSortedAndExcludesTrash(t *testing.T) {
	t.Skip("write path lands in Task 12")
	v, k := newVault(t)
	for _, p := range [][]string{
		{"api", "zeta"},
		{"api", "alpha"},
		{"db", "primary"},
	} {
		if err := SetField(v, k, p[:1], p[1], "password", "value"); err != nil {
			t.Fatal(err)
		}
	}
	if err := Trash(v, k, []string{"api"}, "zeta"); err != nil {
		t.Fatalf("Trash: %v", err)
	}

	h, _ := Open(v, k)
	defer h.Close()
	got, err := h.ListEntries()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"api/alpha", "db/primary"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListEntries = %v, want %v", got, want)
	}
}
