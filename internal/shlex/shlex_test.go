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
