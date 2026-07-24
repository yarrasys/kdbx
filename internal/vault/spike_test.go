package vault

import (
	"os"
	"testing"

	"github.com/tobischo/gokeepasslib/v3"
	w "github.com/tobischo/gokeepasslib/v3/wrappers"
)

// TestSpikeOpenRealPythonVault is the M0 go/no-go gate: the Go engine must open
// a vault+keyfile produced by the Python implementation, read a protected value,
// write a new entry with a protected custom property, and re-open it.
func TestSpikeOpenRealPythonVault(t *testing.T) {
	vaultPath := os.Getenv("KDBX_SPIKE_VAULT")
	keyPath := os.Getenv("KDBX_SPIKE_KEYFILE")
	if vaultPath == "" || keyPath == "" {
		t.Skip("set KDBX_SPIKE_VAULT and KDBX_SPIKE_KEYFILE to run the interop spike")
	}

	creds, err := gokeepasslib.NewKeyCredentials(keyPath)
	if err != nil {
		t.Fatalf("NewKeyCredentials on a Python-minted v2 keyfile: %v", err)
	}

	f, err := os.Open(vaultPath)
	if err != nil {
		t.Fatalf("open vault: %v", err)
	}
	defer f.Close()

	db := gokeepasslib.NewDatabase()
	db.Credentials = creds
	if err := gokeepasslib.NewDecoder(f).Decode(db); err != nil {
		t.Fatalf("decode Python-created vault: %v", err)
	}
	if !db.Header.IsKdbx4() {
		t.Fatalf("expected KDBX4, got header %v", db.Header.Signature)
	}
	if err := db.UnlockProtectedEntries(); err != nil {
		t.Fatalf("unlock: %v", err)
	}

	t.Logf("groups at root: %d", len(db.Content.Root.Groups))
	for _, g := range db.Content.Root.Groups {
		t.Logf("group %q entries=%d subgroups=%d", g.Name, len(g.Entries), len(g.Groups))
		for _, sub := range g.Groups {
			t.Logf("  subgroup %q entries=%d", sub.Name, len(sub.Entries))
			for _, e := range sub.Entries {
				t.Logf("    entry %q values=%d", e.GetTitle(), len(e.Values))
				for _, v := range e.Values {
					t.Logf("      key=%q protected=%v", v.Key, v.Value.Protected.Bool)
				}
			}
		}
	}
	t.Logf("RecycleBinEnabled=%v RecycleBinUUID=%x",
		db.Content.Meta.RecycleBinEnabled.Bool, db.Content.Meta.RecycleBinUUID)

	// Write path: add an entry with a protected custom property.
	e := gokeepasslib.NewEntry()
	e.Times = gokeepasslib.NewTimeData()
	e.Values = append(e.Values,
		gokeepasslib.ValueData{Key: "Title", Value: gokeepasslib.V{Content: "kdbx-spike"}},
		gokeepasslib.ValueData{Key: "Password", Value: gokeepasslib.V{
			Content: "spike-value", Protected: w.NewBoolWrapper(true)}},
		gokeepasslib.ValueData{Key: "CUSTOM_TOKEN", Value: gokeepasslib.V{
			Content: "custom-value", Protected: w.NewBoolWrapper(true)}},
	)
	db.Content.Root.Groups[0].Entries = append(db.Content.Root.Groups[0].Entries, e)

	out := t.TempDir() + "/spike.kdbx"
	of, err := os.Create(out)
	if err != nil {
		t.Fatalf("create out: %v", err)
	}
	if err := db.LockProtectedEntries(); err != nil {
		t.Fatalf("lock: %v", err)
	}
	if err := gokeepasslib.NewEncoder(of).Encode(db); err != nil {
		t.Fatalf("encode: %v", err)
	}
	of.Close()

	// Re-open what we wrote.
	rf, err := os.Open(out)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer rf.Close()
	creds2, _ := gokeepasslib.NewKeyCredentials(keyPath)
	db2 := gokeepasslib.NewDatabase()
	db2.Credentials = creds2
	if err := gokeepasslib.NewDecoder(rf).Decode(db2); err != nil {
		t.Fatalf("decode round-trip: %v", err)
	}
	if err := db2.UnlockProtectedEntries(); err != nil {
		t.Fatalf("unlock round-trip: %v", err)
	}
	var found *gokeepasslib.Entry
	for i := range db2.Content.Root.Groups[0].Entries {
		if db2.Content.Root.Groups[0].Entries[i].GetTitle() == "kdbx-spike" {
			found = &db2.Content.Root.Groups[0].Entries[i]
		}
	}
	if found == nil {
		t.Fatal("spike entry missing after round-trip")
	}
	if got := found.GetContent("CUSTOM_TOKEN"); got != "custom-value" {
		t.Fatalf("protected custom property round-trip: got %q, want %q", got, "custom-value")
	}
	t.Logf("SPIKE OK: %s round-tripped through gokeepasslib", vaultPath)
	t.Logf("SPIKE OUTPUT VAULT: %s", out)

	// Keep the artifact for the pykeepass/keepassxc-cli verification step.
	if dest := os.Getenv("KDBX_SPIKE_OUT"); dest != "" {
		b, err := os.ReadFile(out)
		if err != nil {
			t.Fatalf("read spike output: %v", err)
		}
		if err := os.WriteFile(dest, b, 0o600); err != nil {
			t.Fatalf("persist spike output: %v", err)
		}
		t.Logf("SPIKE OUTPUT PERSISTED: %s", dest)
	}
}
