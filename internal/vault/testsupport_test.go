package vault

import (
	"path/filepath"
	"testing"
)

// newVault creates an empty vault + keyfile in a temp dir and returns their paths.
func newVault(t *testing.T) (vaultPath, keyPath string) {
	t.Helper()
	dir := t.TempDir()
	vaultPath = filepath.Join(dir, "dev.kdbx")
	keyPath = filepath.Join(dir, "dev.keyx")
	if err := Create(vaultPath, keyPath); err != nil {
		t.Fatalf("Create: %v", err)
	}
	return vaultPath, keyPath
}
