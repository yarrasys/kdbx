package kdbxerr

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestConstructorsCarryDocumentedCodes(t *testing.T) {
	cases := []struct {
		err  *Error
		code int
		kind string
	}{
		{NotFound("nope"), 2, "NotFound"},
		{Locked("nope"), 3, "Locked"},
		{NotConfirmed("nope"), 4, "NotConfirmed"},
		{Drift("nope"), 5, "Drift"},
		{Changed("nope"), 6, "VaultChanged"},
		{Preflight("nope"), 7, "Preflight"},
		{Runtime("nope"), 1, "Runtime"},
	}
	for _, c := range cases {
		if CodeOf(c.err) != c.code {
			t.Errorf("%s: code %d, want %d", c.kind, CodeOf(c.err), c.code)
		}
		if KindOf(c.err) != c.kind {
			t.Errorf("kind %q, want %q", KindOf(c.err), c.kind)
		}
	}
}

func TestCodeOfPlainErrorIsOne(t *testing.T) {
	if got := CodeOf(errors.New("boom")); got != 1 {
		t.Fatalf("got %d, want 1", got)
	}
	if got := CodeOf(nil); got != 0 {
		t.Fatalf("nil should be 0, got %d", got)
	}
}

func TestWrappedErrorIsUnwrappable(t *testing.T) {
	base := errors.New("underlying")
	e := Wrap(base, "Locked", 3, "opening vault")
	if !errors.Is(e, base) {
		t.Fatal("errors.Is should find the wrapped error")
	}
	if CodeOf(e) != 3 {
		t.Fatalf("code %d, want 3", CodeOf(e))
	}
}

func TestReportScrubsDetailWithoutDebug(t *testing.T) {
	t.Setenv("KDBX_DEBUG", "")
	var buf bytes.Buffer
	Report(&buf, "get", Wrap(errors.New("SUPER-SECRET-VALUE"), "NotFound", 2, "entry missing"))
	got := buf.String()
	if got != "kdbx: get failed: NotFound\n" {
		t.Fatalf("got %q, want the single scrubbed line", got)
	}
	if strings.Contains(got, "SUPER-SECRET") {
		t.Fatal("scrubbed output leaked error detail")
	}
}

func TestReportIncludesDetailWithDebug(t *testing.T) {
	t.Setenv("KDBX_DEBUG", "1")
	var buf bytes.Buffer
	Report(&buf, "get", fmt.Errorf("detailed context"))
	if !strings.Contains(buf.String(), "detailed context") {
		t.Fatalf("KDBX_DEBUG should reveal detail, got %q", buf.String())
	}
}
