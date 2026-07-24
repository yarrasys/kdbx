// Package runner spawns the child process for `kdbx run`, injecting resolved
// secrets into its environment and passing its exit status straight through
// (spec C5).
package runner

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/yarrasys/kdbx/internal/kdbxerr"
)

// Lookup resolves name against PATH. On Windows exec.LookPath already honors
// PATHEXT, so a .bat/.cmd shim resolves correctly — the failure mode that bit
// the Python implementation.
func Lookup(name string) (string, error) {
	p, err := exec.LookPath(name)
	if err != nil {
		return "", kdbxerr.Wrap(err, "NotFound", 2, "command not found: %s", name)
	}
	return p, nil
}

// Run executes argv with inject merged over the parent environment and returns
// the child's exit code.
func Run(argv []string, inject map[string]string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if len(argv) == 0 {
		return 0, kdbxerr.NotFound("kdbx run: no command given (use: run -- <cmd> ...)")
	}
	exe, err := Lookup(argv[0])
	if err != nil {
		return 0, err
	}

	cmd := exec.Command(exe, argv[1:]...)
	cmd.Env = mergeEnv(os.Environ(), inject)
	cmd.Stdin = orDefault(stdin, os.Stdin)
	cmd.Stdout = orDefaultW(stdout, os.Stdout)
	cmd.Stderr = orDefaultW(stderr, os.Stderr)

	if err := cmd.Start(); err != nil {
		return 0, kdbxerr.Wrap(err, "Runtime", 1, "starting %s", argv[0])
	}

	// Forward interrupts so Ctrl-C reaches the child, not just kdbx.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case s := <-sigs:
				_ = cmd.Process.Signal(s)
			case <-done:
				return
			}
		}
	}()

	err = cmd.Wait()
	close(done)
	signal.Stop(sigs)

	if err == nil {
		return 0, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode(), nil
	}
	return 0, kdbxerr.Wrap(err, "Runtime", 1, "waiting for %s", argv[0])
}

// mergeEnv layers overrides on top of base, replacing existing assignments.
func mergeEnv(base []string, overrides map[string]string) []string {
	if len(overrides) == 0 {
		return base
	}
	out := make([]string, 0, len(base)+len(overrides))
	seen := map[string]bool{}
	for _, kv := range base {
		name := kv
		if i := indexByte(kv, '='); i >= 0 {
			name = kv[:i]
		}
		if v, ok := overrides[name]; ok {
			if seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, name+"="+v)
			continue
		}
		out = append(out, kv)
	}
	for k, v := range overrides {
		if !seen[k] {
			out = append(out, k+"="+v)
		}
	}
	return out
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func orDefault(r io.Reader, def io.Reader) io.Reader {
	if r == nil {
		return def
	}
	return r
}

func orDefaultW(w io.Writer, def io.Writer) io.Writer {
	if w == nil {
		return def
	}
	return w
}
