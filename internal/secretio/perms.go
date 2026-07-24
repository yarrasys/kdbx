//go:build !windows

package secretio

import (
	"os"

	"github.com/yarrasys/kdbx/internal/kdbxerr"
)

// RestrictPerms makes path readable and writable only by its owner.
func RestrictPerms(path string) error {
	if err := os.Chmod(path, 0o600); err != nil {
		return kdbxerr.Wrap(err, "Runtime", 1, "restricting permissions on %s", path)
	}
	return nil
}
