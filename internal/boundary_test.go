package internal_test

import (
	"go/build"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestOnlyVaultImportsTheEngine enforces spec D2: internal/vault is the single
// swap point for the KDBX engine. Any other package importing gokeepasslib is a
// design regression, not a style nit.
func TestOnlyVaultImportsTheEngine(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	const engine = "github.com/tobischo/gokeepasslib"

	err = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		name := info.Name()
		if name == ".git" || name == "dist" || name == "interop" || name == "testdata" {
			return filepath.SkipDir
		}
		pkg, perr := build.ImportDir(p, 0)
		if perr != nil {
			return nil // not a Go package directory
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		for _, imp := range append(pkg.Imports, pkg.TestImports...) {
			if !strings.HasPrefix(imp, engine) {
				continue
			}
			if rel == "internal/vault" {
				continue
			}
			t.Errorf("engine boundary violated: %s imports %s (only internal/vault may)", rel, imp)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
