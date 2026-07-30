package shlex

import (
	"strings"
	"testing"
)

// isShellSpace mirrors the whitespace set Split actually treats as a separator.
// strings.Fields cannot be used directly for the comparison below because it
// splits on all of unicode.IsSpace — \v, \f, U+00A0 — which Split deliberately
// treats as ordinary characters.
func isShellSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

// FuzzSplit checks two things about the command-line splitter that the MCP
// server feeds untrusted-shaped strings into.
//
// First, it must never panic: `kdbx guard` reads a hook payload through this, and
// a panic there fails the guard open on malformed input.
//
// Second, on any input with no quoting or escaping, it must agree exactly with
// plain whitespace splitting. That pins the boring case, which is the one every
// real caller hits, and it catches a quoting bug that only manifests as words
// being silently merged or dropped — the failure mode that would make the guard
// inspect the wrong argv.
func FuzzSplit(f *testing.F) {
	for _, seed := range []string{
		"",
		"kdbx list",
		`sh -c "exit 3"`,
		`echo 'single quoted'`,
		`a\ b`,
		`mixed "double" 'single' plain`,
		`trailing\`,
		`"unterminated`,
		`'unterminated`,
		`empty ""`,
		`nested "she said \"hi\""`,
		`dollar "\$HOME" backtick "\` + "`" + `"`,
		"tabs\tand\nnewlines\r\n",
		`cat ~/.config/keepassxc/demo/dev.keyx`,
		strings.Repeat("a ", 40),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, s string) {
		words, err := Split(s)
		if err != nil {
			// Unterminated quotes and trailing backslashes are errors by design —
			// silently truncating a command would be worse.
			return
		}

		if strings.ContainsAny(s, `'"\`) {
			return // quoting is in play; the simple equivalence below does not hold
		}
		want := strings.FieldsFunc(s, isShellSpace)
		if len(words) != len(want) {
			t.Fatalf("unquoted input split into %d words, want %d\ninput: %q\ngot:   %q\nwant:  %q",
				len(words), len(want), s, words, want)
		}
		for i := range want {
			if words[i] != want[i] {
				t.Fatalf("word %d differs on unquoted input: got %q want %q\ninput: %q",
					i, words[i], want[i], s)
			}
		}
	})
}
