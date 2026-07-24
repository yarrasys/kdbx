package dotenv

import (
	"reflect"
	"testing"

	"github.com/yarrasys/kdbx/internal/kdbxerr"
)

func TestRenderQuotesAndEscapes(t *testing.T) {
	got := Render(
		[]string{"SIMPLE", "WITH_QUOTE", "WITH_BACKSLASH", "WITH_NEWLINE"},
		map[string]string{
			"SIMPLE":         "value",
			"WITH_QUOTE":     `a"b`,
			"WITH_BACKSLASH": `a\b`,
			"WITH_NEWLINE":   "a\nb",
		},
	)
	want := "SIMPLE=\"value\"\n" +
		"WITH_QUOTE=\"a\\\"b\"\n" +
		"WITH_BACKSLASH=\"a\\\\b\"\n" +
		"WITH_NEWLINE=\"a\\nb\"\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestRenderPreservesGivenOrder(t *testing.T) {
	got := Render([]string{"Z", "A"}, map[string]string{"A": "1", "Z": "2"})
	if got != "Z=\"2\"\nA=\"1\"\n" {
		t.Fatalf("order not preserved: %q", got)
	}
}

func TestParseReadsSimpleAndQuotedValues(t *testing.T) {
	vals, order, err := Parse("A=1\nB=\"two\"\n# comment\nC='three'\n")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"A": "1", "B": "two", "C": "three"}
	if !reflect.DeepEqual(vals, want) {
		t.Fatalf("got %v, want %v", vals, want)
	}
	if !reflect.DeepEqual(order, []string{"A", "B", "C"}) {
		t.Fatalf("order %v", order)
	}
}

func TestParseDoesNotInterpolate(t *testing.T) {
	t.Setenv("OUTER", "leaked")
	vals, _, err := Parse("A=\"$OUTER\"\nB=\"${OUTER}\"\n")
	if err != nil {
		t.Fatal(err)
	}
	if vals["A"] != "$OUTER" || vals["B"] != "${OUTER}" {
		t.Fatalf("interpolation happened: %v", vals)
	}
}

// The parser is hand-rolled (godotenv expands ${VAR} in double-quoted values),
// so the cases below cover the syntax python-dotenv accepts.

func TestParseHandlesExportInlineCommentsAndCRLF(t *testing.T) {
	vals, order, err := Parse("export A=1\r\nB=two   # trailing comment\r\nC=\"a # b\"\r\n")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"A": "1", "B": "two", "C": "a # b"}
	if !reflect.DeepEqual(vals, want) {
		t.Fatalf("got %v, want %v", vals, want)
	}
	if !reflect.DeepEqual(order, []string{"A", "B", "C"}) {
		t.Fatalf("order %v", order)
	}
}

func TestParseSingleQuotesOnlyUnescapeBackslashAndQuote(t *testing.T) {
	vals, _, err := Parse(`A='a\nb'` + "\n" + `B='it\'s'` + "\n" + `C='c\\d'` + "\n")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"A": `a\nb`, "B": `it's`, "C": `c\d`}
	if !reflect.DeepEqual(vals, want) {
		t.Fatalf("got %#v, want %#v", vals, want)
	}
}

func TestParseKeepsMultilineQuotedValues(t *testing.T) {
	vals, _, err := Parse("KEY=\"line1\nline2\"\nNEXT=ok\n")
	if err != nil {
		t.Fatal(err)
	}
	if vals["KEY"] != "line1\nline2" || vals["NEXT"] != "ok" {
		t.Fatalf("got %#v", vals)
	}
}

func TestParseLastDuplicateWinsAndOrderKeepsFirstPosition(t *testing.T) {
	vals, order, err := Parse("A=1\nB=2\nA=3\n")
	if err != nil {
		t.Fatal(err)
	}
	if vals["A"] != "3" {
		t.Fatalf("last duplicate should win, got %q", vals["A"])
	}
	if !reflect.DeepEqual(order, []string{"A", "B"}) {
		t.Fatalf("order %v", order)
	}
}

func TestParseSkipsMalformedLines(t *testing.T) {
	vals, order, err := Parse("not a binding\nA=1\n\n   \nB=\n")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(vals, map[string]string{"A": "1", "B": ""}) {
		t.Fatalf("got %#v", vals)
	}
	if !reflect.DeepEqual(order, []string{"A", "B"}) {
		t.Fatalf("order %v", order)
	}
}

func TestParseRejectsUnterminatedQuote(t *testing.T) {
	_, _, err := Parse("A=\"never closed\n")
	if err == nil {
		t.Fatal("want an error for an unterminated quoted value")
	}
	if got := kdbxerr.CodeOf(err); got != 7 {
		t.Fatalf("exit code %d, want 7", got)
	}
}

func TestRoundTrip(t *testing.T) {
	order := []string{"A", "B"}
	items := map[string]string{"A": `weird "value" \ here`, "B": "line1\nline2"}
	vals, _, err := Parse(Render(order, items))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(vals, items) {
		t.Fatalf("round-trip lost data:\ngot  %q\nwant %q", vals, items)
	}
}
