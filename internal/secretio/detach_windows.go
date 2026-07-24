//go:build windows

package secretio

import "os/exec"

// detach is a no-op on Windows: a started process already survives the parent.
func detach(c *exec.Cmd) {}
