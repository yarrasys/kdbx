// Package paths resolves the KeePassXC config directory and expands pointer paths.
package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var syncRoots = []string{"OneDrive", "Dropbox", "iCloud", "iCloudDrive", "Nextcloud", "Google Drive"}

// KeepassxcDir is the base directory holding <project>/<env>.{kdbx,keyx}.
func KeepassxcDir() string {
	if v := os.Getenv("KEEPASSXC_DIR"); v != "" {
		return filepath.Clean(v)
	}
	if runtime.GOOS == "windows" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			base = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
		return filepath.Join(base, "keepassxc")
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "keepassxc")
}

// Expand resolves a pointer path: ${KEEPASSXC_DIR} token, then ~, then absolutize.
func Expand(raw string) (string, error) {
	s := strings.ReplaceAll(raw, "${KEEPASSXC_DIR}", KeepassxcDir())
	if s == "~" || strings.HasPrefix(s, "~/") || strings.HasPrefix(s, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		s = filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(s, "~"), string(os.PathSeparator)))
	}
	return Resolve(s), nil
}

// Resolve returns p absolute with symlinks resolved, matching Python's
// pathlib.Path.resolve() in its default non-strict mode.
//
// Matching matters for compatibility, not tidiness. The Python implementation
// resolves both the pointer's location and every artifact path, so a repo or
// vault reached through a symlink must produce the SAME string here — otherwise
// the two implementations derive different project names (and therefore
// different default vault paths) from the same directory, and take out
// different lock files for the same vault.
//
// filepath.EvalSymlinks fails outright on a path that does not exist, which a
// vault does not until `init` creates it. So resolve the longest existing
// ancestor and re-append the remaining components unchanged, exactly as
// non-strict resolve() does.
func Resolve(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	abs = filepath.Clean(abs)

	cur, rest := abs, ""
	for {
		if resolved, err := filepath.EvalSymlinks(cur); err == nil {
			if rest == "" {
				return resolved
			}
			return filepath.Join(resolved, rest)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return abs // nothing along this path exists; nothing to resolve
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		cur = parent
	}
}

// UnderSyncRoot returns the name of a cloud-sync root present in p, or "".
func UnderSyncRoot(p string) string {
	parts := map[string]bool{}
	for _, seg := range strings.Split(filepath.ToSlash(filepath.Clean(p)), "/") {
		parts[seg] = true
	}
	for _, root := range syncRoots {
		if parts[root] {
			return root
		}
	}
	if parts["AppData"] && parts["Roaming"] {
		return "AppData/Roaming"
	}
	return ""
}
