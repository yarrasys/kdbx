package dotenv

import "testing"

// FuzzParseRoundTrip asserts Parse and Render are inverses: anything Parse
// accepts, Render must write back in a form Parse reads identically.
//
// This is the highest-consequence property in the package. A break here is not a
// crash, it is silent secret corruption — `kdbx export` followed by `kdbx import`
// would change a credential's value without anyone noticing. The parser is
// hand-rolled (see docs/spike-notes.md for why), so it gets fuzzed rather than
// sampled: the interesting inputs are quoting and escaping edge cases nobody
// thinks to write a table entry for.
func FuzzParseRoundTrip(f *testing.F) {
	for _, seed := range []string{
		"",
		"A=1\n",
		"export A=1\n",
		"# comment\nA=\"x y\"\n",
		"A='single'\nB=\"esc\\\"aped\"\n",
		"A=trailing   # comment\n",
		"A=\"spans\nlines\"\n",
		"EMPTY=\n",
		"DUPE=1\nDUPE=2\n",
		"  SPACED   =   value  \n",
		"A=$HOME\n",       // no interpolation: stays literal
		"A=\"\\\\\"\n",    // value is a single backslash
		"A=\"\\n\"\n",     // escaped newline
		"A=back\\slash\n", // unquoted backslash
		"export\tB=tabbed\n",
		"A=1\r\nB=2\r\n",
		"'quoted key'=x\n",
		"A==double-equals\n",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, text string) {
		vals, order, err := Parse(text)
		if err != nil {
			// Rejecting input is a valid outcome — an unterminated quote is a
			// deliberate error. Only accepted input has to round-trip.
			return
		}

		// order is the render order for vals, so the two must agree exactly.
		if len(order) != len(vals) {
			t.Fatalf("order has %d keys, vals has %d: order=%q", len(order), len(vals), order)
		}
		seen := make(map[string]bool, len(order))
		for _, k := range order {
			if _, ok := vals[k]; !ok {
				t.Fatalf("order lists %q, absent from vals", k)
			}
			if seen[k] {
				t.Fatalf("order lists %q twice", k)
			}
			seen[k] = true
		}

		rendered := Render(order, vals)
		vals2, order2, err := Parse(rendered)
		if err != nil {
			t.Fatalf("Parse rejected our own Render output %q: %v", rendered, err)
		}
		if len(order2) != len(order) {
			t.Fatalf("key count changed %d -> %d\ninput:    %q\nrendered: %q",
				len(order), len(order2), text, rendered)
		}
		for i, k := range order {
			if order2[i] != k {
				t.Fatalf("key order changed at %d: %q -> %q\nrendered: %q",
					i, k, order2[i], rendered)
			}
			if vals2[k] != vals[k] {
				t.Fatalf("value for %q changed across round trip:\n  before: %q\n  after:  %q\nrendered: %q",
					k, vals[k], vals2[k], rendered)
			}
		}
	})
}
