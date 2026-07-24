package vault

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/yarrasys/kdbx/internal/kdbxerr"
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
	v, k := newVault(t)
	if err := SetField(v, k, []string{"api"}, "x", "password", "   "); err == nil {
		t.Fatal("SetField must refuse a whitespace-only value")
	}
}

func TestListEntriesIsSortedAndExcludesTrash(t *testing.T) {
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

func TestConcurrentSetsDoNotCorruptTheVault(t *testing.T) {
	v, k := newVault(t)
	done := make(chan error, 4)
	for i := 0; i < 4; i++ {
		go func(n int) {
			done <- SetField(v, k, []string{"api"}, "entry"+string(rune('a'+n)), "password", "value")
		}(i)
	}
	failures := 0
	for i := 0; i < 4; i++ {
		if err := <-done; err != nil {
			failures++ // a losing writer may legitimately see VaultChanged (exit 6)
		}
	}
	h, err := Open(v, k)
	if err != nil {
		t.Fatalf("vault unreadable after concurrent writes: %v", err)
	}
	defer h.Close()
	entries, err := h.ListEntries()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries)+failures != 4 {
		t.Fatalf("entries=%d failures=%d, want them to sum to 4", len(entries), failures)
	}
}

// A rename inside one group is the case that catches Move aliasing the source
// entry's Values: the retitled copy must be the only entry left, and it must
// still decrypt to the original secret.
func TestMoveWithinAGroupRenamesExactlyOneEntry(t *testing.T) {
	v, k := newVault(t)
	if err := SetField(v, k, []string{"db"}, "primary", "password", "sk-test-value"); err != nil {
		t.Fatalf("SetField: %v", err)
	}
	if err := Move(v, k, "db/primary", "db/main"); err != nil {
		t.Fatalf("Move: %v", err)
	}
	h, err := Open(v, k)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer h.Close()
	got, err := h.ListEntries()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"db/main"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ListEntries = %v, want %v", got, want)
	}
	val, err := h.GetField([]string{"db"}, "main", "password")
	if err != nil {
		t.Fatalf("GetField after move: %v", err)
	}
	if val != "sk-test-value" {
		t.Fatalf("value after move = %q, want %q", val, "sk-test-value")
	}
}

func TestMoveAcrossGroupsKeepsTheValue(t *testing.T) {
	v, k := newVault(t)
	if err := SetField(v, k, []string{"api"}, "openai", "password", "sk-one"); err != nil {
		t.Fatalf("SetField: %v", err)
	}
	if err := Move(v, k, "api/openai", "vendors/openai"); err != nil {
		t.Fatalf("Move: %v", err)
	}
	h, err := Open(v, k)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer h.Close()
	got, err := h.ListEntries()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"vendors/openai"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ListEntries = %v, want %v", got, want)
	}
	val, err := h.GetField([]string{"vendors"}, "openai", "password")
	if err != nil {
		t.Fatalf("GetField after move: %v", err)
	}
	if val != "sk-one" {
		t.Fatalf("value after move = %q, want %q", val, "sk-one")
	}
}

func TestMoveOfAMissingEntryIsNotFound(t *testing.T) {
	v, k := newVault(t)
	err := Move(v, k, "api/nope", "api/other")
	if err == nil {
		t.Fatal("moving a missing entry must fail")
	}
	if kdbxerr.CodeOf(err) != 2 {
		t.Fatalf("exit code = %d, want 2 (NotFound)", kdbxerr.CodeOf(err))
	}
}
