package secretio

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/yarrasys/kdbx/internal/kdbxerr"
)

// DefaultClipboardClear is how long a copied secret lives on the clipboard.
const DefaultClipboardClear = 15 * time.Second

// ClipboardCommand returns the platform's copy command, or nil if none applies.
func ClipboardCommand() []string {
	switch {
	case runtime.GOOS == "darwin":
		return []string{"pbcopy"}
	case runtime.GOOS == "windows":
		return []string{"powershell", "-NoProfile", "-Command", "Set-Clipboard"}
	case os.Getenv("WAYLAND_DISPLAY") != "":
		return []string{"wl-copy"}
	case os.Getenv("DISPLAY") != "":
		return []string{"xclip", "-selection", "clipboard"}
	}
	return nil
}

// ClipboardCopy places value on the clipboard and schedules a detached clear.
func ClipboardCopy(value string, clearAfter time.Duration) error {
	argv := ClipboardCommand()
	if argv == nil {
		return kdbxerr.Runtime("no clipboard backend available")
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = strings.NewReader(value)
	if err := cmd.Run(); err != nil {
		return kdbxerr.Wrap(err, "Runtime", 1, "copying to clipboard")
	}
	if clearAfter <= 0 {
		return nil
	}
	self, err := os.Executable()
	if err != nil {
		return nil // copied successfully; we simply cannot schedule the clear
	}
	// Not named "clear": that would shadow the Go 1.21 builtin.
	clearCmd := exec.Command(self, "internal-clear-clip",
		"--after", strconv.Itoa(int(clearAfter/time.Second)))
	detach(clearCmd)
	if err := clearCmd.Start(); err != nil {
		return nil // copied successfully; we simply cannot schedule the clear
	}
	_ = clearCmd.Process.Release()
	return nil
}

// ClearClipboardAfter sleeps for seconds, then blanks the clipboard.
func ClearClipboardAfter(seconds int) error {
	time.Sleep(time.Duration(seconds) * time.Second)
	argv := ClipboardCommand()
	if argv == nil {
		return nil
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = strings.NewReader("")
	_ = cmd.Run()
	return nil
}
