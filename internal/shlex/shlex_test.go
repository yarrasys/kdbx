package shlex

import (
	"reflect"
	"testing"
)

func TestSplitMatchesPythonShlex(t *testing.T) {
	// Expectations below were produced by python3 -c
	// "import shlex,json;print(json.dumps(shlex.split(INPUT)))" — see
	// TestSplitAgreesWithPythonShlex for the live differential check.
	cases := []struct {
		in   string
		want []string
	}{
		{`npm test`, []string{"npm", "test"}},
		{`sh -c "exit 3"`, []string{"sh", "-c", "exit 3"}},
		{`sh -c 'exit 3'`, []string{"sh", "-c", "exit 3"}},
		{`echo "a b" c`, []string{"echo", "a b", "c"}},
		{`echo 'it'"'"'s'`, []string{"echo", "it's"}},
		{`echo a\ b`, []string{"echo", "a b"}},
		{`echo "say \"hi\""`, []string{"echo", `say "hi"`}},
		{`echo "back\\slash"`, []string{"echo", `back\slash`}},
		{`echo "keep\nliteral"`, []string{"echo", `keep\nliteral`}},
		{`  spaced   out  `, []string{"spaced", "out"}},
		{``, nil},
		{`echo ""`, []string{"echo", ""}},
		{`pytest -k "not slow" -v`, []string{"pytest", "-k", "not slow", "-v"}},
		{`echo "$HOME"`, []string{"echo", "$HOME"}},
	}
	for _, c := range cases {
		got, err := Split(c.in)
		if err != nil {
			t.Errorf("Split(%q) errored: %v", c.in, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("Split(%q) = %#v, want %#v", c.in, got, c.want)
		}
	}
}

func TestSplitRejectsUnterminatedQuotes(t *testing.T) {
	for _, bad := range []string{`echo "unterminated`, `echo 'unterminated`, `echo trailing\`} {
		if _, err := Split(bad); err == nil {
			t.Errorf("Split(%q) should have failed", bad)
		}
	}
}

// TestSplitPreservesInvalidUTF8 pins byte-preservation. Split converted to []rune
// until FuzzSplit found "\xdd": that conversion replaces every byte which is not
// valid UTF-8 with U+FFFD, so re-encoding produced EF BF BD and the word handed to
// exec was not the word the caller passed. Silent argument rewriting is the wrong
// failure mode for a package that turns a string into an argv — an unterminated
// quote errors rather than truncate, and this should be no different.
func TestSplitPreservesInvalidUTF8(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want []string
	}{
		{"bare invalid byte", "\xdd", []string{"\xdd"}},
		{"invalid byte in a word", "a\xddb", []string{"a\xddb"}},
		{"invalid byte, two words", "a\xdd b\xfe", []string{"a\xdd", "b\xfe"}},
		{"quoted invalid byte", "\"a\xddb\"", []string{"a\xddb"}},
		{"single-quoted invalid byte", "'\xff'", []string{"\xff"}},
		{"valid multi-byte is untouched", "café 日本語", []string{"café", "日本語"}},
		{"quoted multi-byte", `"héllo wörld"`, []string{"héllo wörld"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Split(tc.in)
			if err != nil {
				t.Fatalf("Split(%q): %v", tc.in, err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("Split(%q) = %q, want %q", tc.in, got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("word %d = %q (% x), want %q (% x)",
						i, got[i], got[i], tc.want[i], tc.want[i])
				}
			}
		})
	}
}
