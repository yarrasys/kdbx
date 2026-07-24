package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
	"github.com/yarrasys/kdbx/cmd"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"kdbx": cmd.Execute,
	}))
}

func TestScripts(t *testing.T) {
	// $NOPOINTER backs the "no pointer anywhere" cases. $WORK cannot serve: a
	// script's own txtar archive writes .keepassxc.json at the work-dir root, so
	// any directory under $WORK discovers it by walking up. The guard below keeps
	// the case honest — a stray pointer file above the temp dir on some machine
	// would otherwise turn a real assertion into a silent pass.
	nowhere := t.TempDir()
	if found := pointerAbove(t, nowhere); found != "" {
		t.Fatalf("%s is not pointer-free: %s exists", nowhere, found)
	}

	testscript.Run(t, testscript.Params{
		Dir: "testdata/script",
		Setup: func(e *testscript.Env) error {
			e.Setenv("KEEPASSXC_DIR", e.WorkDir+"/kpxc")
			e.Setenv("KDBX_ENV", "")
			e.Setenv("HOME", e.WorkDir)
			e.Setenv("NOPOINTER", nowhere)
			return nil
		},
	})
}

// pointerAbove returns the first .keepassxc.json at or above dir, or "".
func pointerAbove(t *testing.T, dir string) string {
	t.Helper()
	cur, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("resolving %s: %v", dir, err)
	}
	for {
		if cand := filepath.Join(cur, ".keepassxc.json"); fileExists(cand) {
			return cand
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return ""
		}
		cur = parent
	}
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
