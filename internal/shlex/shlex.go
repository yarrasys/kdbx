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
func Split(s string) ([]string, error) {
	var (
		words []string
		cur   []rune
		has   bool // a word has begun, so "" is a real empty argument
	)

	flush := func() {
		if has {
			words = append(words, string(cur))
			cur = cur[:0]
			has = false
		}
	}

	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch c {
		case ' ', '\t', '\n', '\r':
			flush()

		case '\\':
			// Outside quotes a backslash escapes the next character verbatim.
			if i+1 >= len(runes) {
				return nil, fmt.Errorf("shlex: trailing backslash in %q", s)
			}
			i++
			cur = append(cur, runes[i])
			has = true

		case '\'':
			// Single quotes are fully literal — no escapes inside.
			has = true
			j := i + 1
			for ; j < len(runes) && runes[j] != '\''; j++ {
				cur = append(cur, runes[j])
			}
			if j >= len(runes) {
				return nil, fmt.Errorf("shlex: unterminated single quote in %q", s)
			}
			i = j

		case '"':
			// Inside double quotes only \" \\ \$ \` are escapes; every other
			// backslash stays literal, as in a POSIX shell.
			has = true
			j := i + 1
			for ; j < len(runes) && runes[j] != '"'; j++ {
				if runes[j] == '\\' && j+1 < len(runes) {
					switch runes[j+1] {
					case '"', '\\', '$', '`':
						j++
					}
				}
				cur = append(cur, runes[j])
			}
			if j >= len(runes) {
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
