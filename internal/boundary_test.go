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
		// XTestImports matters as much as the other two: an external test package
		// (package foo_test) importing the engine would otherwise slip through and
		// quietly establish a second place that knows the engine's types.
		all := append(append(append([]string{}, pkg.Imports...), pkg.TestImports...), pkg.XTestImports...)
		for _, imp := range all {
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
