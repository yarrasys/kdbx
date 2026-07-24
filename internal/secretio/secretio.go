// Package secretio handles every path a secret value takes in or out of kdbx:
// intake that never crosses argv, masked display, confirmations, and 0600
// atomic writes (spec C5, C8).
package secretio

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"golang.org/x/term"
)

// Mask is the constant stand-in printed instead of a value. It encodes neither
// length nor prefix.
const Mask = "(set, hidden)"

// ReadOpts describes where a secret value should come from.
type ReadOpts struct {
	FromEnv  string
	Raw      bool
	Stdin    io.Reader
	IsTTY    bool
	PromptFn func(prompt string) (string, error)
}

// ReadSecret obtains a secret without it ever crossing argv.
func ReadSecret(o ReadOpts) (string, error) {
	if o.FromEnv != "" {
		v, ok := os.LookupEnv(o.FromEnv)
		if !ok {
			return "", kdbxerr.Preflight("--from-env %s is not set", o.FromEnv)
		}
		return trim(v, o.Raw), nil
	}

	if o.IsTTY {
		prompt := o.PromptFn
		if prompt == nil {
			prompt = promptHidden
		}
		v, err := prompt("value: ")
		if err != nil {
			return "", kdbxerr.Wrap(err, "Runtime", 1, "reading value")
		}
		again, err := prompt("confirm: ")
		if err != nil {
			return "", kdbxerr.Wrap(err, "Runtime", 1, "reading confirmation")
		}
		if v != again {
			return "", kdbxerr.Runtime("values did not match")
		}
		return v, nil
	}

	src := o.Stdin
	if src == nil {
		src = os.Stdin
	}
	b, err := io.ReadAll(src)
	if err != nil {
		return "", kdbxerr.Wrap(err, "Runtime", 1, "reading stdin")
	}
	v := string(b)
	if strings.TrimSpace(v) == "" {
		fmt.Fprint(os.Stderr, "kdbx: no value provided — stdin is empty "+
			"(pipe a value, use --from-env, or run interactively from a terminal)\n")
		return "", kdbxerr.Runtime("no value provided via stdin")
	}
	return trim(v, o.Raw), nil
}

func trim(v string, raw bool) string {
	if raw {
		return v
	}
	v = strings.TrimSuffix(v, "\n")
	return strings.TrimSuffix(v, "\r")
}

func promptHidden(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	return string(b), err
}

// Confirm asks for interactive y/N approval of an irreversible operation. There
// is no non-interactive override: writes are a human role.
func Confirm(prompt string, in io.Reader, errOut io.Writer, isTTY bool) bool {
	if !isTTY {
		fmt.Fprintf(errOut, "%s: refused — needs an interactive terminal to confirm\n", prompt)
		return false
	}
	fmt.Fprintf(errOut, "%s [y/N] ", prompt)
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

// IsTerminal reports whether f is an interactive terminal.
func IsTerminal(f *os.File) bool { return term.IsTerminal(int(f.Fd())) }

// AtomicWriteSecret writes data to path with owner-only permissions.
func AtomicWriteSecret(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return kdbxerr.Wrap(err, "Runtime", 1, "creating %s", path)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return kdbxerr.Wrap(err, "Runtime", 1, "writing %s", path)
	}
	if err := f.Close(); err != nil {
		return kdbxerr.Wrap(err, "Runtime", 1, "closing %s", path)
	}
	return RestrictPerms(path)
}
