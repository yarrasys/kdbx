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
	abs, err := filepath.Abs(s)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
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
