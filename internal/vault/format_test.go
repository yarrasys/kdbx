package vault

import (
	"encoding/binary"
	"os"
	"testing"
)

// The KDBX file header opens with two magic words and a packed version:
//
//	bytes 0-3   signature 1, always 0x9AA2D903
//	bytes 4-7   signature 2, 0xB54BFB67 for KDBX 2/3/4
//	bytes 8-9   minor version, uint16 little-endian
//	bytes 10-11 major version, uint16 little-endian
const (
	kdbxSig1     = 0x9AA2D903
	kdbxSig2     = 0xB54BFB67
	wantMajor    = 4
	wantMinor    = 0
	headerPrefix = 12
)

// TestCreateWritesKdbx40OnDisk asserts the on-disk format by reading raw bytes
// rather than asking the engine what it thinks it wrote. This is a compatibility
// guard, not a unit test of Create: the README and the design spec both promise
// vaults that `keepassxc-cli` and the KeePassXC desktop app open directly, and
// KDBX 4.0 is the version that promise was verified against (docs/spike-notes.md).
//
// gokeepasslib gained opt-in KDBX 4.1 support in v3.7.0, where
// WithDatabaseKDBXVersion4 still delegates to WithDatabaseKDBXVersion40. If a
// future engine bump changes that default — or someone swaps the constructor for
// WithDatabaseKDBXVersion41 — every vault kdbx writes silently changes format.
// That must be a deliberate, documented decision, so it fails here first.
func TestCreateWritesKdbx40OnDisk(t *testing.T) {
	v, _ := newVault(t)

	raw, err := os.ReadFile(v)
	if err != nil {
		t.Fatalf("reading vault: %v", err)
	}
	if len(raw) < headerPrefix {
		t.Fatalf("vault is %d bytes, too short to hold a KDBX header", len(raw))
	}

	if got := binary.LittleEndian.Uint32(raw[0:4]); got != kdbxSig1 {
		t.Errorf("signature 1 = %#x, want %#x", got, kdbxSig1)
	}
	if got := binary.LittleEndian.Uint32(raw[4:8]); got != kdbxSig2 {
		t.Errorf("signature 2 = %#x, want %#x", got, kdbxSig2)
	}

	minor := binary.LittleEndian.Uint16(raw[8:10])
	major := binary.LittleEndian.Uint16(raw[10:12])
	if major != wantMajor || minor != wantMinor {
		t.Errorf("on-disk KDBX version = %d.%d, want %d.%d — changing the vault "+
			"format is a compatibility break; update the spec and spike notes first",
			major, minor, wantMajor, wantMinor)
	}
}
