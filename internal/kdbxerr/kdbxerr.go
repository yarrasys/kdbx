// Package kdbxerr carries a stable error "kind" and the documented exit code
// (spec C6) alongside an error, and reports failures without leaking detail
// that could contain a secret (spec C7).
package kdbxerr

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
)

// Error is an error that knows its exit code and its user-visible kind.
type Error struct {
	Kind string
	Code int
	Msg  string
	Err  error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Msg, e.Err)
	}
	return e.Msg
}

func (e *Error) Unwrap() error { return e.Err }

func newErr(kind string, code int, format string, args ...any) *Error {
	return &Error{Kind: kind, Code: code, Msg: fmt.Sprintf(format, args...)}
}

// NotFound — pointer, env, entry, field, or required argument missing (exit 2).
func NotFound(format string, args ...any) *Error { return newErr("NotFound", 2, format, args...) }

// Locked — vault locked, keyfile missing, credentials rejected, lock timeout (exit 3).
func Locked(format string, args ...any) *Error { return newErr("Locked", 3, format, args...) }

// NotConfirmed — a destructive operation was not confirmed (exit 4).
func NotConfirmed(format string, args ...any) *Error {
	return newErr("NotConfirmed", 4, format, args...)
}

// Drift — a mapped var does not resolve (exit 5).
func Drift(format string, args ...any) *Error { return newErr("Drift", 5, format, args...) }

// Changed — the vault changed underneath a read-modify-write (exit 6).
func Changed(format string, args ...any) *Error { return newErr("VaultChanged", 6, format, args...) }

// Preflight — bad input caught before touching the vault (exit 7).
func Preflight(format string, args ...any) *Error { return newErr("Preflight", 7, format, args...) }

// Runtime — anything else (exit 1).
func Runtime(format string, args ...any) *Error { return newErr("Runtime", 1, format, args...) }

// Wrap attaches a kind and code to an existing error.
func Wrap(err error, kind string, code int, format string, args ...any) *Error {
	return &Error{Kind: kind, Code: code, Msg: fmt.Sprintf(format, args...), Err: err}
}

// CodeOf returns the exit code for err: 0 if nil, the carried code for a *Error,
// otherwise 1.
func CodeOf(err error) int {
	if err == nil {
		return 0
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return 1
}

// KindOf returns the stable kind name for err.
func KindOf(err error) string {
	if err == nil {
		return ""
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return "Runtime"
}

// Report writes the single scrubbed failure line for op, plus full detail when
// KDBX_DEBUG is set.
func Report(w io.Writer, op string, err error) {
	if err == nil {
		return
	}
	if os.Getenv("KDBX_DEBUG") != "" {
		fmt.Fprintf(w, "kdbx: %s failed: %v\n%s", op, err, debug.Stack())
		return
	}
	fmt.Fprintf(w, "kdbx: %s failed: %s\n", op, KindOf(err))
}
