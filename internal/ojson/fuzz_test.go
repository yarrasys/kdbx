package ojson

import (
	"bytes"
	"testing"
)

// FuzzParseMarshalIdempotent asserts that re-encoding an already-encoded object
// changes nothing. This package exists so that `kdbx set --var`, `mv` and `import`
// can rewrite a committed `.keepassxc.json` without churning the diff, so drift on
// a second write — a reordered key, a re-escaped character — is the exact bug it
// was written to prevent, and it would only show up in someone's git history.
func FuzzParseMarshalIdempotent(f *testing.F) {
	for _, seed := range []string{
		`{}`,
		`{"a":1}`,
		`{"b":2,"a":1}`,
		`{"nested":{"deep":{"x":"y"}}}`,
		`{"project":"p","defaultEnv":"dev","envs":{"dev":{"vault":"v","keyFile":"k"}}}`,
		`{"dupe":1,"dupe":2}`,
		`{"unicode":"café"}`,
		`{"emoji":"🔐"}`,
		`{"esc":"quote\" backslash\\ newline\n"}`,
		`{"num":1.5e10,"big":123456789012345678901234567890}`,
		`{"null":null,"true":true,"arr":[1,{"k":"v"},null]}`,
		`{"":"empty key"}`,
		`{"spaced"   :   "value"}`,
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, b []byte) {
		// Cap the input size. A pointer file is a few hundred bytes; letting the
		// engine grow inputs into the megabytes buys no new structure and starves
		// the workers, since decoding copies every value as a RawMessage.
		if len(b) > 32*1024 {
			return
		}
		o, err := Parse(b)
		if err != nil {
			return // rejecting non-object or malformed JSON is correct
		}

		first, err := o.MarshalJSON()
		if err != nil {
			// Encoding can legitimately fail (e.g. invalid UTF-8 in a key); that
			// surfaces as an error rather than corruption, which is the contract.
			return
		}

		o2, err := Parse(first)
		if err != nil {
			t.Fatalf("Parse rejected our own MarshalJSON output %q: %v", first, err)
		}
		second, err := o2.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON failed on a re-parse of its own output %q: %v", first, err)
		}
		if !bytes.Equal(first, second) {
			t.Fatalf("re-encoding is not idempotent:\n  first:  %q\n  second: %q", first, second)
		}

		// Key order is the whole point of the package, so assert it survives.
		if k1, k2 := o.Keys(), o2.Keys(); len(k1) != len(k2) {
			t.Fatalf("key count changed %d -> %d", len(k1), len(k2))
		} else {
			for i := range k1 {
				if k1[i] != k2[i] {
					t.Fatalf("key order changed at %d: %q -> %q", i, k1[i], k2[i])
				}
			}
		}

		// Indent is what actually lands on disk; it must stay parseable.
		indented, err := o.Indent()
		if err != nil {
			return
		}
		if _, err := Parse(indented); err != nil {
			t.Fatalf("Parse rejected our own Indent output %q: %v", indented, err)
		}
	})
}
