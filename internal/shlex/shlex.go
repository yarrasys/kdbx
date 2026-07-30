// Package shlex splits a command line into words the way a POSIX shell does,
// matching Python's shlex.split(posix=True).
//
// It exists because the MCP server receives a command as a single string and
// must turn it into an argv. Splitting on whitespace alone would mis-execute
// anything containing a quoted argument — `sh -c "exit 3"` would become four
// words instead of three. The Python implementation this ports uses
// shlex.split, so matching it is also a compatibility requirement.
package shlex

import "fmt"

// Split parses s into words, honoring single quotes, double quotes, and
// backslash escapes. An unterminated quote is an error rather than a silent
// truncation.
//
// Splitting is byte-oriented, like a real shell. It used to convert to []rune
// first, which silently replaced every byte that is not valid UTF-8 with U+FFFD —
// so an argument containing such a byte was executed as something other than what
// the caller asked for, with no error. (Found by FuzzSplit on the input "\xdd".)
// Working in bytes is also safe for multi-byte characters: every character this
// function treats specially is ASCII, and UTF-8 continuation bytes are all
// 0x80-0xBF, so they can never be mistaken for a delimiter, quote or backslash.
func Split(s string) ([]string, error) {
	var (
		words []string
		cur   []byte
		has   bool // a word has begun, so "" is a real empty argument
	)

	flush := func() {
		if has {
			words = append(words, string(cur))
			cur = cur[:0]
			has = false
		}
	}

	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case ' ', '\t', '\n', '\r':
			flush()

		case '\\':
			// Outside quotes a backslash escapes the next character verbatim.
			if i+1 >= len(s) {
				return nil, fmt.Errorf("shlex: trailing backslash in %q", s)
			}
			i++
			cur = append(cur, s[i])
			has = true

		case '\'':
			// Single quotes are fully literal — no escapes inside.
			has = true
			j := i + 1
			for ; j < len(s) && s[j] != '\''; j++ {
				cur = append(cur, s[j])
			}
			if j >= len(s) {
				return nil, fmt.Errorf("shlex: unterminated single quote in %q", s)
			}
			i = j

		case '"':
			// Inside double quotes only \" \\ \$ \` are escapes; every other
			// backslash stays literal, as in a POSIX shell.
			has = true
			j := i + 1
			for ; j < len(s) && s[j] != '"'; j++ {
				if s[j] == '\\' && j+1 < len(s) {
					switch s[j+1] {
					case '"', '\\', '$', '`':
						j++
					}
				}
				cur = append(cur, s[j])
			}
			if j >= len(s) {
				return nil, fmt.Errorf("shlex: unterminated double quote in %q", s)
			}
			i = j

		default:
			cur = append(cur, c)
			has = true
		}
	}
	flush()
	return words, nil
}
