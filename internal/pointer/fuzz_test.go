package pointer

import (
	"strings"
	"testing"
)

// FuzzParseEntryPath asserts the invariants every caller of the entry-path
// grammar (spec C2) relies on. `internal/vault` walks groupPath to find a group
// and then looks up title, so an empty component or an empty title would send it
// looking for a nameless group — which is how a lookup silently resolves to the
// wrong entry rather than failing. Those must be errors, never accepted output.
func FuzzParseEntryPath(f *testing.F) {
	for _, seed := range []string{
		"api/openai",
		"api/openai:password",
		"Title",
		"a/b/c/d:custom_field",
		"",
		"/leading",
		"trailing/",
		"double//slash",
		"a:b:c",
		"empty:",
		":onlyfield",
		"unicode/café:pässword",
		"spaces in name/and title",
		strings.Repeat("a/", 50) + "leaf",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw string) {
		groupPath, title, field, err := ParseEntryPath(raw)
		if err != nil {
			// Every rejection path must return nothing usable, so a caller that
			// ignores err cannot act on half-parsed values.
			if groupPath != nil || title != "" || field != "" {
				t.Fatalf("error path returned values: groupPath=%q title=%q field=%q for %q",
					groupPath, title, field, raw)
			}
			return
		}

		if title == "" {
			t.Fatalf("accepted %q with an empty title", raw)
		}
		if field == "" {
			t.Fatalf("accepted %q with an empty field", raw)
		}
		for i, seg := range groupPath {
			if seg == "" {
				t.Fatalf("accepted %q with empty group component at %d", raw, i)
			}
		}
		// A ':' inside the group path or title means the field split went wrong.
		if strings.Contains(title, ":") {
			t.Fatalf("title %q from %q still contains ':'", title, raw)
		}
		for _, seg := range groupPath {
			if strings.Contains(seg, ":") {
				t.Fatalf("group component %q from %q still contains ':'", seg, raw)
			}
		}
	})
}
