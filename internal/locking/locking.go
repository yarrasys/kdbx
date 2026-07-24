// Package locking prevents two kdbx writes from racing and detects a vault that
// changed underneath a read-modify-write (spec C9).
package locking

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"time"

	"github.com/gofrs/flock"
	"github.com/yarrasys/kdbx/internal/kdbxerr"
)

// LockTimeout bounds how long a write waits for a competing kdbx process.
const LockTimeout = 10 * time.Second

// WithVaultLock runs fn while holding the advisory lock for vaultPath.
func WithVaultLock(vaultPath string, fn func() error) error {
	lock := flock.New(vaultPath + ".lock")
	ctx, cancel := context.WithTimeout(context.Background(), LockTimeout)
	defer cancel()

	got, err := lock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		return kdbxerr.Wrap(err, "Locked", 3, "acquiring the vault lock")
	}
	if !got {
		return kdbxerr.Locked("another kdbx process holds the vault lock; try again")
	}
	defer func() { _ = lock.Unlock() }()
	return fn()
}

// CaptureState returns a content hash of the vault, or "" if it does not exist.
func CaptureState(vaultPath string) (string, error) {
	b, err := os.ReadFile(vaultPath)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", kdbxerr.Wrap(err, "Runtime", 1, "reading the vault for integrity capture")
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// VerifyUnchanged fails when the vault differs from its captured state.
func VerifyUnchanged(vaultPath, captured string) error {
	now, err := CaptureState(vaultPath)
	if err != nil {
		return err
	}
	if now != captured {
		return kdbxerr.Changed("vault changed underneath us; re-run")
	}
	return nil
}
