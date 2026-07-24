//go:build windows

package secretio

import (
	"os"
	"os/exec"
)

// RestrictPerms strips inherited ACLs and grants the current user full control.
// Failure is not fatal: the file already exists with default ACLs, and a hard
// failure here would make kdbx unusable on locked-down machines.
func RestrictPerms(path string) error {
	user := os.Getenv("USERNAME")
	if user == "" {
		return nil
	}
	cmd := exec.Command("icacls", path, "/inheritance:r", "/grant:r", user+":F")
	_ = cmd.Run()
	return nil
}
