// Package vault is the ONLY package permitted to import the KDBX engine
// (spec D2). Its public interface uses plain types exclusively, so swapping the
// engine touches this package alone.
package vault

import (
	"os"
	"sort"
	"strings"

	"github.com/tobischo/gokeepasslib/v3"
	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/keyfile"
	"github.com/yarrasys/kdbx/internal/secretio"
)

var reserved = map[string]string{
	"title":    "Title",
	"username": "UserName",
	"password": "Password",
	"url":      "URL",
	"notes":    "Notes",
}

// zeroUUID is the all-zero UUID a vault reports for RecycleBinUUID when the bin
// is enabled but has never been created (see docs/spike-notes.md).
var zeroUUID gokeepasslib.UUID

// Handle is an opened, unlocked database.
type Handle struct {
	db        *gokeepasslib.Database
	vaultPath string
	keyPath   string
}

// Create writes a new KDBX4+Argon2 vault and mints its keyfile. It refuses if
// either artifact already exists.
func Create(vaultPath, keyPath string) error {
	if _, err := os.Stat(vaultPath); err == nil {
		return kdbxerr.Preflight("refusing to overwrite an existing vault: %s", vaultPath)
	}
	if _, err := os.Stat(keyPath); err == nil {
		return kdbxerr.Preflight("refusing to overwrite an existing keyfile: %s", keyPath)
	}
	if err := os.MkdirAll(dirOf(vaultPath), 0o700); err != nil {
		return kdbxerr.Wrap(err, "Runtime", 1, "creating vault directory")
	}
	if err := os.MkdirAll(dirOf(keyPath), 0o700); err != nil {
		return kdbxerr.Wrap(err, "Runtime", 1, "creating keyfile directory")
	}
	if err := keyfile.Mint(keyPath); err != nil {
		return err
	}

	creds, err := gokeepasslib.NewKeyCredentials(keyPath)
	if err != nil {
		return kdbxerr.Wrap(err, "Locked", 3, "building credentials from %s", keyPath)
	}
	// Explicitly 4.0, not the deprecated WithDatabaseKDBXVersion4 alias: the engine
	// gained opt-in 4.1 support in v3.7.0, and the format keepassxc-cli and the
	// desktop app were verified against is 4.0 (docs/spike-notes.md). Naming the
	// minor version keeps that a decision rather than whatever the alias resolves
	// to next. TestCreateWritesKdbx40OnDisk pins the on-disk bytes.
	db := gokeepasslib.NewDatabase(gokeepasslib.WithDatabaseKDBXVersion40())
	db.Credentials = creds
	root := gokeepasslib.NewGroup()
	root.Name = "Root"
	db.Content.Root = &gokeepasslib.RootData{Groups: []gokeepasslib.Group{root}}

	return writeDB(db, vaultPath)
}

// Open decodes and unlocks the vault at vaultPath using keyPath.
func Open(vaultPath, keyPath string) (*Handle, error) {
	if err := keyfile.Validate(keyPath); err != nil {
		return nil, err
	}
	// Read the keyfile ourselves rather than gokeepasslib.NewKeyCredentials:
	// its ParseKeyFile opens the file but never closes it, and that leaked
	// handle blocks TempDir cleanup (and any delete) on Windows.
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, kdbxerr.Wrap(err, "Locked", 3, "reading keyfile %s", keyPath)
	}
	creds, err := gokeepasslib.NewKeyDataCredentials(keyData)
	if err != nil {
		return nil, kdbxerr.Wrap(err, "Locked", 3, "building credentials from %s", keyPath)
	}
	f, err := os.Open(vaultPath)
	if err != nil {
		return nil, kdbxerr.Wrap(err, "NotFound", 2, "opening vault %s", vaultPath)
	}
	defer func() { _ = f.Close() }() // read-only; a close error here is not meaningful

	db := gokeepasslib.NewDatabase()
	db.Credentials = creds
	if err := gokeepasslib.NewDecoder(f).Decode(db); err != nil {
		return nil, kdbxerr.Wrap(err, "Locked", 3, "decrypting vault %s", vaultPath)
	}
	if !db.Header.IsKdbx4() {
		return nil, kdbxerr.Locked("vault %s is not KDBX4", vaultPath)
	}
	if err := db.UnlockProtectedEntries(); err != nil {
		return nil, kdbxerr.Wrap(err, "Locked", 3, "unlocking protected entries")
	}
	return &Handle{db: db, vaultPath: vaultPath, keyPath: keyPath}, nil
}

// Close releases the handle. It never writes.
func (h *Handle) Close() error { h.db = nil; return nil }

// GetField reads one field of one entry.
func (h *Handle) GetField(groupPath []string, title, field string) (string, error) {
	e := h.findEntry(groupPath, title, false)
	if e == nil {
		return "", kdbxerr.NotFound("entry not found: %s", joinPath(groupPath, title))
	}
	key := field
	if native, ok := reserved[strings.ToLower(field)]; ok {
		key = native
	}
	val := e.GetContent(key)
	if val == "" {
		return "", kdbxerr.NotFound(
			"field not set: %s (entry %s exists but '%s' is empty/absent)",
			field, joinPath(groupPath, title), field)
	}
	return val, nil
}

// ListEntries returns every live entry path, sorted, excluding the Recycle Bin.
func (h *Handle) ListEntries() ([]string, error) {
	var out []string
	walk(h.rootGroups(), nil, h.recycleBinName(), func(path []string, e *gokeepasslib.Entry) {
		out = append(out, strings.Join(append(append([]string{}, path...), e.GetTitle()), "/"))
	})
	sort.Strings(out)
	return out, nil
}

func dirOf(p string) string {
	i := strings.LastIndexAny(p, `/\`)
	if i <= 0 {
		return "."
	}
	return p[:i]
}

func joinPath(groupPath []string, title string) string {
	return strings.Join(append(append([]string{}, groupPath...), title), "/")
}

// rootGroups returns the vault's top-level groups, tolerating a database whose
// content or root is absent.
func (h *Handle) rootGroups() []gokeepasslib.Group {
	if h.db == nil || h.db.Content == nil || h.db.Content.Root == nil {
		return nil
	}
	return h.db.Content.Root.Groups
}

// recycleBinName returns the name of the group designated as the Recycle Bin,
// or "" when the vault has none.
func (h *Handle) recycleBinName() string {
	if h.db == nil || h.db.Content == nil {
		return ""
	}
	meta := h.db.Content.Meta
	if meta == nil || !meta.RecycleBinEnabled.Bool {
		return ""
	}
	// A freshly created vault advertises the bin as enabled while its UUID is
	// still all zeros — enabled but not yet created. Treat that as "no bin" so
	// a group whose UUID happens to be zero is never mistaken for the bin.
	if meta.RecycleBinUUID.Compare(zeroUUID) {
		return ""
	}
	for _, g := range h.rootGroups() {
		if g.UUID.Compare(meta.RecycleBinUUID) {
			return g.Name
		}
		for _, sub := range g.Groups {
			if sub.UUID.Compare(meta.RecycleBinUUID) {
				return sub.Name
			}
		}
	}
	return ""
}

// walk visits every entry outside the named recycle-bin group. The path passed
// to fn excludes the synthetic root group, matching Python's e.path semantics.
func walk(groups []gokeepasslib.Group, prefix []string, recycleBin string,
	fn func(path []string, e *gokeepasslib.Entry)) {
	for gi := range groups {
		g := &groups[gi]
		if recycleBin != "" && g.Name == recycleBin && len(prefix) == 0 {
			continue
		}
		path := prefix
		if len(prefix) > 0 || !isRootGroup(g) {
			path = append(append([]string{}, prefix...), g.Name)
		}
		for ei := range g.Entries {
			fn(path, &g.Entries[ei])
		}
		walk(g.Groups, path, recycleBin, fn)
	}
}

// isRootGroup reports whether g is the synthetic top-level container that does
// not appear in entry paths. Confirmed against real Python-created vaults, which
// nest everything under one top-level group literally named "Root"
// (docs/spike-notes.md).
func isRootGroup(g *gokeepasslib.Group) bool { return g.Name == "Root" }

// findEntry locates an entry by group path and title.
func (h *Handle) findEntry(groupPath []string, title string, includeTrash bool) *gokeepasslib.Entry {
	var found *gokeepasslib.Entry
	rb := h.recycleBinName()
	if includeTrash {
		rb = ""
	}
	walk(h.rootGroups(), nil, rb, func(path []string, e *gokeepasslib.Entry) {
		if found != nil {
			return
		}
		if e.GetTitle() != title {
			return
		}
		if len(path) != len(groupPath) {
			if includeTrash && len(groupPath) == 0 {
				found = e
			}
			return
		}
		for i := range path {
			if path[i] != groupPath[i] {
				return
			}
		}
		found = e
	})
	return found
}

// writeDB locks protected entries and writes the database crash-safely
// (spec C3): tmp -> restrict -> rename old to .bak -> rename tmp into place ->
// restrict -> drop .bak.
func writeDB(db *gokeepasslib.Database, vaultPath string) error {
	if err := db.LockProtectedEntries(); err != nil {
		return kdbxerr.Wrap(err, "Runtime", 1, "locking protected entries")
	}
	tmp := vaultPath + ".tmp"
	bak := vaultPath + ".bak"

	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return kdbxerr.Wrap(err, "Runtime", 1, "creating %s", tmp)
	}
	if err := gokeepasslib.NewEncoder(f).Encode(db); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return kdbxerr.Wrap(err, "Runtime", 1, "encoding vault")
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return kdbxerr.Wrap(err, "Runtime", 1, "closing %s", tmp)
	}
	if err := secretio.RestrictPerms(tmp); err != nil {
		return err
	}
	if _, err := os.Stat(vaultPath); err == nil {
		if err := os.Rename(vaultPath, bak); err != nil {
			return kdbxerr.Wrap(err, "Runtime", 1, "backing up %s", vaultPath)
		}
	}
	if err := os.Rename(tmp, vaultPath); err != nil {
		return kdbxerr.Wrap(err, "Runtime", 1, "installing %s", vaultPath)
	}
	if err := secretio.RestrictPerms(vaultPath); err != nil {
		return err
	}
	_ = os.Remove(bak)
	// Re-unlock so the in-memory handle stays usable after a write.
	return db.UnlockProtectedEntries()
}
