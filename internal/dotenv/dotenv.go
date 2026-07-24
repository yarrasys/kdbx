// Package dotenv renders and parses .env files for kdbx export/import. Values
// are always double-quoted on output and never interpolated on input (spec C5).
//
// The parser is hand-rolled rather than delegated to github.com/joho/godotenv:
// that library expands $VAR and ${VAR} inside double-quoted values (v1.5.1
// resolves an unknown name to the empty string), which would silently destroy
// any stored secret containing a dollar sign. The Python reference passes
// interpolate=False to python-dotenv, and the rules below mirror that parser:
// optional `export `, first `=` splits key from value, single-quoted values
// unescape only \\ and \', double-quoted values also unescape \" \a \b \f \n
// \r \t \v, and unquoted values drop an inline ` #` comment and trailing space.
package dotenv

import (
	"strings"

	"github.com/yarrasys/kdbx/internal/kdbxerr"
)

// Render writes one KEY="value" line per name in order.
func Render(order []string, items map[string]string) string {
	var b strings.Builder
	for _, k := range order {
		v, ok := items[k]
		if !ok {
			continue
		}
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(quote(v))
		b.WriteString("\n")
	}
	return b.String()
}

func quote(v string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
	)
	return `"` + r.Replace(v) + `"`
}

// Parse reads dotenv text, returning values and the key order they appeared in.
// No variable interpolation is performed: a value of "$HOME" parses to the
// literal five characters. Malformed lines are skipped, matching python-dotenv;
// an unterminated quoted value is an error, because truncating it would corrupt
// the secret it holds.
func Parse(text string) (map[string]string, []string, error) {
	vals := make(map[string]string)
	var order []string
	for i := 0; i < len(text); {
		if isSpace(text[i]) {
			i++
			continue
		}
		if text[i] == '#' {
			i = skipLine(text, i)
			continue
		}
		i = skipExport(text, i)
		key, next := readKey(text, i)
		i = next
		if key == "" || i >= len(text) || text[i] != '=' {
			i = skipLine(text, i)
			continue
		}
		i++ // consume '='
		for i < len(text) && isBlank(text[i]) {
			i++
		}
		var value string
		var ok bool
		switch {
		case i < len(text) && (text[i] == '"' || text[i] == '\''):
			value, i, ok = readQuoted(text, i)
			if !ok {
				return nil, nil, kdbxerr.Preflight(
					"unterminated quoted value for %s in dotenv input", key)
			}
		default:
			value, i = readUnquoted(text, i)
		}
		i = skipLine(text, i)
		if _, seen := vals[key]; !seen {
			order = append(order, key)
		}
		vals[key] = value
	}
	return vals, order, nil
}

// skipExport consumes a leading `export ` (or `export\t`) if present.
func skipExport(text string, i int) int {
	const kw = "export"
	rest := text[i:]
	if !strings.HasPrefix(rest, kw) || len(rest) <= len(kw) || !isBlank(rest[len(kw)]) {
		return i
	}
	i += len(kw)
	for i < len(text) && isBlank(text[i]) {
		i++
	}
	return i
}

// readKey reads a variable name and any blanks between it and the '='.
func readKey(text string, i int) (string, int) {
	start := i
	for i < len(text) && !isSpace(text[i]) && text[i] != '=' && text[i] != '#' {
		i++
	}
	key := text[start:i]
	for i < len(text) && isBlank(text[i]) {
		i++
	}
	return key, i
}

// readQuoted reads a '- or "-delimited value starting at text[i], returning the
// unescaped value, the index just past the closing quote, and whether it closed.
// Quoted values may span newlines, as in python-dotenv.
func readQuoted(text string, i int) (string, int, bool) {
	q := text[i]
	i++
	var b strings.Builder
	for i < len(text) {
		c := text[i]
		switch {
		case c == '\\' && i+1 < len(text):
			b.WriteString(unescape(q, text[i+1]))
			i += 2
		case c == q:
			return b.String(), i + 1, true
		default:
			b.WriteByte(c)
			i++
		}
	}
	return "", i, false
}

// unescape decodes one backslash escape. Single-quoted values recognise only
// \\ and \'; anything else keeps its backslash, as python-dotenv does.
func unescape(quoteChar, c byte) string {
	if c == '\\' || c == '\'' {
		return string(c)
	}
	if quoteChar == '\'' {
		return `\` + string(c)
	}
	switch c {
	case '"':
		return `"`
	case 'a':
		return "\a"
	case 'b':
		return "\b"
	case 'f':
		return "\f"
	case 'n':
		return "\n"
	case 'r':
		return "\r"
	case 't':
		return "\t"
	case 'v':
		return "\v"
	}
	return `\` + string(c)
}

// readUnquoted reads a bare value to end of line, dropping an inline comment
// and trailing whitespace.
func readUnquoted(text string, i int) (string, int) {
	start := i
	for i < len(text) && text[i] != '\n' && text[i] != '\r' {
		i++
	}
	return strings.TrimRight(stripInlineComment(text[start:i]), " \t\v\f"), i
}

// stripInlineComment removes ` #...` (whitespace then '#') and everything after.
func stripInlineComment(part string) string {
	for j := 0; j < len(part); j++ {
		if !isBlank(part[j]) {
			continue
		}
		k := j
		for k < len(part) && isBlank(part[k]) {
			k++
		}
		if k < len(part) && part[k] == '#' {
			return part[:j]
		}
		j = k - 1
	}
	return part
}

// skipLine advances past the next newline.
func skipLine(text string, i int) int {
	for i < len(text) && text[i] != '\n' {
		i++
	}
	if i < len(text) {
		i++
	}
	return i
}

func isBlank(c byte) bool { return c == ' ' || c == '\t' }

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\v' || c == '\f'
}
