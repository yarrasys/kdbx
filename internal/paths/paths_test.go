package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestKeepassxcDirPrefersOverride(t *testing.T) {
	t.Setenv("KEEPASSXC_DIR", filepath.Join("/custom", "vaults"))
	if got, want := KeepassxcDir(), filepath.Join("/custom", "vaults"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestKeepassxcDirUsesXDGOnUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only default")
	}
	t.Setenv("KEEPASSXC_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	if got, want := KeepassxcDir(), filepath.Join("/xdg", "keepassxc"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestExpandSubstitutesKeepassxcDirToken(t *testing.T) {
	t.Setenv("KEEPASSXC_DIR", "/base")
	got, err := Expand("${KEEPASSXC_DIR}/proj/dev.kdbx")
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if want := filepath.Join("/base", "proj", "dev.kdbx"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestExpandExpandsTilde(t *testing.T) {
	got, err := Expand("~/vaults/dev.kdbx")
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if strings.HasPrefix(got, "~") {
		t.Fatalf("tilde not expanded: %q", got)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("not absolute: %q", got)
	}
}

func TestUnderSyncRootDetectsKnownRoots(t *testing.T) {
	cases := map[string]string{
		filepath.Join("/Users", "n", "OneDrive", "v.kdbx"):           "OneDrive",
		filepath.Join("/Users", "n", "Dropbox", "v.kdbx"):            "Dropbox",
		filepath.Join("/Users", "n", ".config", "kpxc", "v.kdbx"):    "",
		filepath.Join("/c", "Users", "n", "AppData", "Roaming", "x"): "AppData/Roaming",
	}
	for in, want := range cases {
		if got := UnderSyncRoot(in); got != want {
			t.Fatalf("UnderSyncRoot(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveFollowsSymlinksLikePythonResolve(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs elevation on Windows")
	}
	root := t.TempDir()
	real := filepath.Join(root, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if got, want := Resolve(link), Resolve(real); got != want {
		t.Fatalf("Resolve(symlink) = %q, want %q", got, want)
	}
}

func TestResolveHandlesPathsThatDoNotExistYet(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs elevation on Windows")
	}
	root := t.TempDir()
	real := filepath.Join(root, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	// A vault does not exist until `init` creates it, so Resolve must still
	// resolve the existing ancestors rather than failing outright.
	got := Resolve(filepath.Join(link, "proj", "dev.kdbx"))
	want := filepath.Join(Resolve(real), "proj", "dev.kdbx")
	if got != want {
		t.Fatalf("Resolve(nonexistent under symlink) = %q, want %q", got, want)
	}
}
