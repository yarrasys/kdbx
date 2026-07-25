package vault

import (
	"os"
	"strings"

	"github.com/tobischo/gokeepasslib/v3"
	w "github.com/tobischo/gokeepasslib/v3/wrappers"
	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/keyfile"
	"github.com/yarrasys/kdbx/internal/locking"
	"github.com/yarrasys/kdbx/internal/pointer"
)

// recycleBinGroupName is the group KeePassXC uses for soft-deleted entries.
const recycleBinGroupName = "Recycle Bin"

// mutate opens the vault under lock, verifies it has not changed, applies fn,
// and writes it back crash-safely (spec C3, C9).
func mutate(vaultPath, keyPath string, fn func(h *Handle) error) error {
	return locking.WithVaultLock(vaultPath, func() error {
		captured, err := locking.CaptureState(vaultPath)
		if err != nil {
			return err
		}
		h, err := Open(vaultPath, keyPath)
		if err != nil {
			return err
		}
		defer func() { _ = h.Close() }()
		if err := locking.VerifyUnchanged(vaultPath, captured); err != nil {
			return err
		}
		if err := fn(h); err != nil {
			return err
		}
		return writeDB(h.db, vaultPath)
	})
}

// SetField stores value at the given entry+field, creating groups and the entry
// as needed. Non-reserved fields become protected custom properties.
func SetField(vaultPath, keyPath string, groupPath []string, title, field, value string) error {
	if strings.TrimSpace(value) == "" {
		return kdbxerr.Preflight("refusing to write empty value — provide a non-empty secret")
	}
	return mutate(vaultPath, keyPath, func(h *Handle) error {
		grp := h.ensureGroup(groupPath)
		e := h.findEntry(groupPath, title, false)
		if e == nil {
			ne := gokeepasslib.NewEntry()
			ne.Times = gokeepasslib.NewTimeData()
			ne.Values = append(ne.Values, gokeepasslib.ValueData{
				Key: "Title", Value: gokeepasslib.V{Content: title},
			})
			grp.Entries = append(grp.Entries, ne)
			e = &grp.Entries[len(grp.Entries)-1]
		}
		key := field
		protected := true
		if native, ok := reserved[strings.ToLower(field)]; ok {
			key = native
			protected = native == "Password"
		}
		setValue(e, key, value, protected)
		return nil
	})
}

// setValue writes or replaces one key on an entry.
func setValue(e *gokeepasslib.Entry, key, value string, protected bool) {
	for i := range e.Values {
		if e.Values[i].Key == key {
			e.Values[i].Value = gokeepasslib.V{
				Content: value, Protected: w.NewBoolWrapper(protected),
			}
			return
		}
	}
	e.Values = append(e.Values, gokeepasslib.ValueData{
		Key: key, Value: gokeepasslib.V{Content: value, Protected: w.NewBoolWrapper(protected)},
	})
}

// ensureGroup walks (creating as needed) to the group holding an entry.
func (h *Handle) ensureGroup(groupPath []string) *gokeepasslib.Group {
	root := h.rootGroup()
	cur := root
	for _, name := range groupPath {
		var next *gokeepasslib.Group
		for i := range cur.Groups {
			if cur.Groups[i].Name == name {
				next = &cur.Groups[i]
				break
			}
		}
		if next == nil {
			ng := gokeepasslib.NewGroup()
			ng.Name = name
			cur.Groups = append(cur.Groups, ng)
			next = &cur.Groups[len(cur.Groups)-1]
		}
		cur = next
	}
	return cur
}

// rootGroup returns the synthetic top-level container, creating it if absent.
func (h *Handle) rootGroup() *gokeepasslib.Group {
	if len(h.db.Content.Root.Groups) == 0 {
		g := gokeepasslib.NewGroup()
		g.Name = "Root"
		h.db.Content.Root.Groups = append(h.db.Content.Root.Groups, g)
	}
	return &h.db.Content.Root.Groups[0]
}

// Trash soft-deletes an entry into the Recycle Bin.
func Trash(vaultPath, keyPath string, groupPath []string, title string) error {
	return mutate(vaultPath, keyPath, func(h *Handle) error {
		if e, _ := h.locate(groupPath, title, false); e == nil {
			return kdbxerr.NotFound("entry not found: %s", joinPath(groupPath, title))
		}
		// ensureRecycleBin may append to the root group's slice of subgroups and
		// reallocate its backing array, so locate the entry *after* it: a pointer
		// taken earlier would address the stale copy and the removal would be lost.
		bin := h.ensureRecycleBin()
		e, owner := h.locate(groupPath, title, false)
		if e == nil {
			return kdbxerr.NotFound("entry not found: %s", joinPath(groupPath, title))
		}
		moved := *e
		removeEntry(owner, title)
		bin.Entries = append(bin.Entries, moved)
		return nil
	})
}

// Purge permanently removes an entry, including one already in the Recycle Bin.
func Purge(vaultPath, keyPath string, groupPath []string, title string) error {
	return mutate(vaultPath, keyPath, func(h *Handle) error {
		e, owner := h.locate(groupPath, title, true)
		if e == nil {
			return kdbxerr.NotFound("entry not found: %s", joinPath(groupPath, title))
		}
		removeEntry(owner, title)
		return nil
	})
}

// Move relocates and/or retitles an entry.
func Move(vaultPath, keyPath, src, dst string) error {
	sg, st, _, err := pointer.ParseEntryPath(src)
	if err != nil {
		return err
	}
	dg, dt, _, err := pointer.ParseEntryPath(dst)
	if err != nil {
		return err
	}
	return mutate(vaultPath, keyPath, func(h *Handle) error {
		e, owner := h.locate(sg, st, false)
		if e == nil {
			return kdbxerr.NotFound("entry not found: %s", joinPath(sg, st))
		}
		// Copy the value slice, not just the entry struct: `moved := *e` shares its
		// Values backing array with the entry still in the tree, so retitling the
		// copy renames the original too. removeEntry (which matches on title) then
		// finds nothing, leaving the vault with two entries under the destination
		// title whose shared protected values get locked twice on write — they
		// decrypt to nothing, so the moved secret reads back as unset.
		moved := *e
		moved.Values = append([]gokeepasslib.ValueData(nil), e.Values...)
		setValue(&moved, "Title", dt, false)
		removeEntry(owner, st)
		target := h.ensureGroup(dg)
		target.Entries = append(target.Entries, moved)
		return nil
	})
}

// Rekey mints newKeyPath and re-encrypts the vault under it.
//
// It deliberately does NOT remove oldKeyPath — the caller installs the new key
// by renaming it over the live keyfile. Deleting the old key here would make
// every failure path unrecoverable, since the keyfile is the vault's sole
// credential. On error the NEW key is removed instead, leaving the vault
// readable by the old one.
func Rekey(vaultPath, oldKeyPath, newKeyPath string) error {
	if err := keyfile.Mint(newKeyPath); err != nil {
		return err
	}
	err := locking.WithVaultLock(vaultPath, func() error {
		h, err := Open(vaultPath, oldKeyPath)
		if err != nil {
			return err
		}
		defer func() { _ = h.Close() }()
		creds, err := gokeepasslib.NewKeyCredentials(newKeyPath)
		if err != nil {
			return kdbxerr.Wrap(err, "Locked", 3, "building credentials from the new keyfile")
		}
		h.db.Credentials = creds
		return writeDB(h.db, vaultPath)
	})
	if err != nil {
		_ = os.Remove(newKeyPath)
		return err
	}
	return nil
}

// locate finds an entry and the group that owns it.
func (h *Handle) locate(groupPath []string, title string, includeTrash bool) (*gokeepasslib.Entry, *gokeepasslib.Group) {
	var (
		hit   *gokeepasslib.Entry
		owner *gokeepasslib.Group
	)
	rb := h.recycleBinName()
	if includeTrash {
		rb = ""
	}
	var visit func(groups []gokeepasslib.Group, prefix []string)
	visit = func(groups []gokeepasslib.Group, prefix []string) {
		for gi := range groups {
			g := &groups[gi]
			if rb != "" && g.Name == rb && len(prefix) == 0 {
				continue
			}
			path := prefix
			if len(prefix) > 0 || !isRootGroup(g) {
				path = append(append([]string{}, prefix...), g.Name)
			}
			for ei := range g.Entries {
				if hit != nil {
					return
				}
				if g.Entries[ei].GetTitle() != title {
					continue
				}
				if !includeTrash && !samePath(path, groupPath) {
					continue
				}
				hit, owner = &g.Entries[ei], g
				return
			}
			visit(g.Groups, path)
		}
	}
	visit(h.rootGroups(), nil)
	return hit, owner
}

func samePath(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// removeEntry deletes the first entry with the given title from g.
func removeEntry(g *gokeepasslib.Group, title string) {
	if g == nil {
		return
	}
	for i := range g.Entries {
		if g.Entries[i].GetTitle() == title {
			g.Entries = append(g.Entries[:i], g.Entries[i+1:]...)
			return
		}
	}
}

// ensureRecycleBin returns the Recycle Bin group, creating it and recording its
// UUID in Meta on first use, exactly as KeePassXC does.
func (h *Handle) ensureRecycleBin() *gokeepasslib.Group {
	root := h.rootGroup()
	for i := range root.Groups {
		if root.Groups[i].Name == recycleBinGroupName {
			h.db.Content.Meta.RecycleBinEnabled = w.NewBoolWrapper(true)
			h.db.Content.Meta.RecycleBinUUID = root.Groups[i].UUID
			return &root.Groups[i]
		}
	}
	bin := gokeepasslib.NewGroup()
	bin.Name = recycleBinGroupName
	root.Groups = append(root.Groups, bin)
	created := &root.Groups[len(root.Groups)-1]
	h.db.Content.Meta.RecycleBinEnabled = w.NewBoolWrapper(true)
	h.db.Content.Meta.RecycleBinUUID = created.UUID
	return created
}
