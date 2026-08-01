// Package allowlist matches a child argv against the pointer's `run.allow`
// list (spec C1, C5). Matching is exact: each entry is shell-split once and
// must equal the argv element for element. No prefix matching, deliberately —
// a `pytest` prefix would admit `pytest --pdb`, a REPL holding the injected
// secrets.
package allowlist

import (
	"slices"

	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/shlex"
)

// Match reports whether argv exactly equals one of the allow entries after
// shell-splitting each entry. An entry that cannot be split is an error, not
// a skip: a malformed allowlist must fail closed and loudly.
func Match(allow []string, argv []string) (bool, error) {
	if len(argv) == 0 {
		return false, nil
	}
	for i, entry := range allow {
		want, err := shlex.Split(entry)
		if err != nil {
			return false, kdbxerr.Wrap(err, "Preflight", 7,
				"run.allow[%d] is not a valid command line", i)
		}
		if slices.Equal(want, argv) {
			return true, nil
		}
	}
	return false, nil
}
