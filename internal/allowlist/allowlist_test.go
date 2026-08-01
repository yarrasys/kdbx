package allowlist

import (
	"errors"
	"testing"

	"github.com/yarrasys/kdbx/internal/kdbxerr"
)

func TestMatchExactArgv(t *testing.T) {
	allow := []string{"npm test", "pytest -q"}
	for _, tc := range []struct {
		argv []string
		want bool
	}{
		{[]string{"npm", "test"}, true},
		{[]string{"pytest", "-q"}, true},
		{[]string{"npm", "test", "--watch"}, false}, // no prefix matching
		{[]string{"npm"}, false},
		{[]string{"pytest"}, false}, // "pytest -q" does not cover bare pytest
		{[]string{"env"}, false},
		{nil, false},
	} {
		got, err := Match(allow, tc.argv)
		if err != nil {
			t.Fatalf("Match(%v): %v", tc.argv, err)
		}
		if got != tc.want {
			t.Errorf("Match(%v) = %v, want %v", tc.argv, got, tc.want)
		}
	}
}

func TestMatchQuotedEntry(t *testing.T) {
	// Entries are shell-split, so quoting groups words exactly like argv does.
	got, err := Match([]string{`sh -c "echo hi"`}, []string{"sh", "-c", "echo hi"})
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Fatal("quoted entry did not match its argv")
	}
}

func TestMatchEmptyListMatchesNothing(t *testing.T) {
	if got, _ := Match([]string{}, []string{"env"}); got {
		t.Fatal("empty allowlist matched")
	}
}

func TestMatchUnsplittableEntryIsAnError(t *testing.T) {
	_, err := Match([]string{`npm "test`}, []string{"npm", "test"})
	if err == nil {
		t.Fatal("unbalanced quote in an entry must error, not be skipped")
	}
	var ke *kdbxerr.Error
	if !errors.As(err, &ke) || ke.Kind != "Preflight" {
		t.Fatalf("want a Preflight kdbxerr, got %v", err)
	}
}
