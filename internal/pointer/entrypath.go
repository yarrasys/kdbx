package pointer

import (
	"regexp"
	"strings"

	"github.com/yarrasys/kdbx/internal/kdbxerr"
)

var varNameRe = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

// ValidVarName reports whether name is a legal environment-variable name for a
// pointer var mapping (spec C2).
func ValidVarName(name string) bool { return varNameRe.MatchString(name) }

// ParseEntryPath splits "group/sub/Title[:field]" into its parts. The field
// defaults to "password". A name component may not contain ':' and no component
// may be empty (spec C2).
func ParseEntryPath(raw string) (groupPath []string, title, field string, err error) {
	field = "password"
	body := raw
	if i := strings.LastIndex(raw, ":"); i >= 0 {
		body, field = raw[:i], raw[i+1:]
		if strings.Contains(body, ":") {
			return nil, "", "", kdbxerr.Preflight("ambiguous path (multiple ':'): %q", raw)
		}
		if field == "" {
			return nil, "", "", kdbxerr.Preflight("empty field: %q", raw)
		}
	}
	segments := strings.Split(body, "/")
	for _, seg := range segments {
		if seg == "" {
			return nil, "", "", kdbxerr.Preflight("empty path component: %q", raw)
		}
	}
	title = segments[len(segments)-1]
	groupPath = segments[:len(segments)-1]
	return groupPath, title, field, nil
}

// EntryOf returns the entry portion of a var mapping value (everything before
// an optional ":field").
func EntryOf(mapping string) string {
	if i := strings.Index(mapping, ":"); i >= 0 {
		return mapping[:i]
	}
	return mapping
}
