//go:build !windows

package secretio

import (
	"os/exec"
	"syscall"
)

// detach puts the helper in its own session so it outlives the parent.
func detach(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
