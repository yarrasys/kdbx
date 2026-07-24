package jsonout

import (
	"bytes"
	"strings"
	"testing"

	"github.com/yarrasys/kdbx/internal/kdbxerr"
)

func TestWriteEmitsCompactJSONWithNewline(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, map[string]any{"ok": true}); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "{\"ok\":true}\n" {
		t.Fatalf("got %q", got)
	}
}

func TestWriteErrorEnvelope(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteError(&buf, "check", kdbxerr.Drift("drifted")); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	for _, want := range []string{`"op":"check"`, `"exit":5`, `"kind":"Drift"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("envelope %q missing %q", got, want)
		}
	}
}
