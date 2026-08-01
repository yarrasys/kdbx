// Package maskio replaces exact secret values in a byte stream with a fixed
// mask, so `kdbx run`'s child output can be captured (by an agent harness, a
// pipe, a log) without carrying the injected values. It is an accident
// barrier, not containment: an encoded value passes through untouched, which
// is the documented tradeoff (spec C5, issue #14).
package maskio

import (
	"bytes"
	"io"
	"sort"
)

// MinLen is the shortest value worth masking. Below it, masking mangles
// ordinary output ("true", "8080") for no real gain.
const MinLen = 8

// Mask is what a matched value becomes. Fixed, so the output leaks neither
// length nor prefix.
var Mask = []byte("***")

// Values extracts the maskable values from a resolved var map: deduplicated,
// short values dropped, longest first so that at any position the longest
// match wins.
func Values(vars map[string]string) []string {
	seen := make(map[string]bool, len(vars))
	var out []string
	for _, v := range vars {
		if len(v) >= MinLen && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i]) != len(out[j]) {
			return len(out[i]) > len(out[j])
		}
		return out[i] < out[j]
	})
	return out
}

// Writer masks values on their way to dst. Call Flush after the last Write:
// the writer holds back bytes that could still grow into a match, and Flush
// resolves them once the stream is known to be over.
type Writer struct {
	dst     io.Writer
	secrets [][]byte // longest first
	buf     []byte
}

// New returns a Writer masking values into dst. Ordering and the MinLen
// cutoff are enforced here rather than trusted from the caller: matchAt is
// only longest-match if the secrets actually are longest-first.
func New(dst io.Writer, values []string) *Writer {
	w := &Writer{dst: dst, secrets: make([][]byte, 0, len(values))}
	for _, v := range values {
		if len(v) >= MinLen {
			w.secrets = append(w.secrets, []byte(v))
		}
	}
	sort.Slice(w.secrets, func(i, j int) bool { return len(w.secrets[i]) > len(w.secrets[j]) })
	return w
}

// Write masks and forwards p. It always accepts all of p; a tail that could
// still become a match is buffered until the next Write or Flush.
func (w *Writer) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	if err := w.scan(false); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Flush emits everything still buffered. A complete value sitting at the end
// of the stream is masked; a partial prefix is ordinary output and passes
// through.
func (w *Writer) Flush() error {
	return w.scan(true)
}

// scan walks the buffer, masking complete matches. Unless final, it stops at
// a tail that is a proper prefix of some secret: matching a shorter secret
// there would emit the mask and then leak the longer secret's remainder when
// the next bytes arrive.
func (w *Writer) scan(final bool) error {
	buf := w.buf
	var out []byte
	i := 0
	for i < len(buf) {
		rem := buf[i:]
		if !final && w.couldGrow(rem) {
			break
		}
		if n := w.matchAt(rem); n > 0 {
			out = append(out, Mask...)
			i += n
			continue
		}
		out = append(out, buf[i])
		i++
	}
	w.buf = append(w.buf[:0], buf[i:]...)
	if len(out) == 0 {
		return nil
	}
	_, err := w.dst.Write(out)
	return err
}

// couldGrow reports whether rem is a proper prefix of some secret, i.e. more
// bytes could still turn it into (or into a longer) match.
func (w *Writer) couldGrow(rem []byte) bool {
	for _, s := range w.secrets {
		if len(s) > len(rem) && bytes.HasPrefix(s, rem) {
			return true
		}
	}
	return false
}

// matchAt returns the length of the longest secret starting at the head of
// rem, or 0. Secrets are sorted longest-first, so the first hit is longest.
func (w *Writer) matchAt(rem []byte) int {
	for _, s := range w.secrets {
		if bytes.HasPrefix(rem, s) {
			return len(s)
		}
	}
	return 0
}
