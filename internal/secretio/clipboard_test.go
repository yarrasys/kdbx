package secretio

import (
	"runtime"
	"testing"
)

func TestClipboardCommandIsPlatformAppropriate(t *testing.T) {
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("DISPLAY", "")
	cmd := ClipboardCommand()
	switch runtime.GOOS {
	case "darwin":
		if len(cmd) == 0 || cmd[0] != "pbcopy" {
			t.Fatalf("darwin should use pbcopy, got %v", cmd)
		}
	case "windows":
		if len(cmd) == 0 || cmd[0] != "powershell" {
			t.Fatalf("windows should use powershell, got %v", cmd)
		}
	default:
		if cmd != nil {
			t.Fatalf("headless linux should have no clipboard backend, got %v", cmd)
		}
	}
}

func TestClipboardCommandPrefersWaylandWhenPresent(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only")
	}
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	if cmd := ClipboardCommand(); len(cmd) == 0 || cmd[0] != "wl-copy" {
		t.Fatalf("got %v, want wl-copy", cmd)
	}
}

func TestClipboardCopyFailsCleanlyWithoutBackend(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only headless case")
	}
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("DISPLAY", "")
	if err := ClipboardCopy("value", 0); err == nil {
		t.Fatal("expected an error when no clipboard backend exists")
	}
}
