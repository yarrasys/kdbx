package maskio

import (
	"bytes"
	"strings"
	"testing"
)

// write pushes chunks through a Writer and returns the masked result.
func write(t *testing.T, values []string, chunks ...string) string {
	t.Helper()
	var out bytes.Buffer
	w := New(&out, values)
	for _, c := range chunks {
		n, err := w.Write([]byte(c))
		if err != nil {
			t.Fatalf("Write(%q): %v", c, err)
		}
		if n != len(c) {
			t.Fatalf("Write(%q) consumed %d of %d", c, n, len(c))
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	return out.String()
}

func TestMasksValueInOneWrite(t *testing.T) {
	got := write(t, []string{"sk-test-value"}, "TOKEN=sk-test-value\n")
	if got != "TOKEN=***\n" {
		t.Fatalf("got %q", got)
	}
}

func TestMasksValueSplitAcrossWrites(t *testing.T) {
	got := write(t, []string{"sk-test-value"}, "TOKEN=sk-te", "st-val", "ue\n")
	if got != "TOKEN=***\n" {
		t.Fatalf("got %q", got)
	}
}

func TestMasksValueAtEndOfStreamOnFlush(t *testing.T) {
	got := write(t, []string{"sk-test-value"}, "TOKEN=sk-test-value")
	if got != "TOKEN=***" {
		t.Fatalf("got %q", got)
	}
}

func TestShortValuesAreNotMasked(t *testing.T) {
	// Masking "true" or "8080" would mangle ordinary output for no gain.
	got := write(t, []string{"8080", "true"}, "PORT=8080 DEBUG=true\n")
	if got != "PORT=8080 DEBUG=true\n" {
		t.Fatalf("got %q", got)
	}
}

func TestMasksEveryValueEverywhere(t *testing.T) {
	got := write(t, []string{"first-value", "second-value"},
		"a=first-value b=second-value c=first-value\n")
	if got != "a=*** b=*** c=***\n" {
		t.Fatalf("got %q", got)
	}
}

// The dangerous shape: one value is a prefix of another. The writer must not
// mask the short one and then leak the long one's tail when more bytes arrive,
// and must still mask the short one when the stream truly ends there.
func TestPrefixValueDoesNotLeakLongerValueTail(t *testing.T) {
	values := []string{"abcdefgh", "abcdefghXY"}
	if got := write(t, values, "v=abcdefgh", "XY\n"); got != "v=***\n" {
		t.Fatalf("longer value split after shorter prefix: got %q", got)
	}
	if got := write(t, values, "v=abcdefgh"); got != "v=***" {
		t.Fatalf("stream ending on the shorter value: got %q", got)
	}
	if got := write(t, values, "v=abcdefghQQ\n"); got != "v=***QQ\n" {
		t.Fatalf("shorter value followed by non-matching bytes: got %q", got)
	}
}

func TestPartialPrefixAtEndOfStreamIsEmittedOnFlush(t *testing.T) {
	// "sk-te" could have grown into the value; once the stream ends it is
	// ordinary output and must not be swallowed.
	got := write(t, []string{"sk-test-value"}, "tail is sk-te")
	if got != "tail is sk-te" {
		t.Fatalf("got %q", got)
	}
}

func TestLargeOutputAroundTheValue(t *testing.T) {
	pad := strings.Repeat("x", 64*1024)
	got := write(t, []string{"sk-test-value"}, pad+"sk-test-value"+pad)
	if got != pad+"***"+pad {
		t.Fatalf("large output mangled (len %d)", len(got))
	}
}

func TestValuesFiltersAndOrders(t *testing.T) {
	vs := Values(map[string]string{
		"A": "short",      // < 8 bytes: dropped
		"B": "abcdefghXY", // kept
		"C": "abcdefgh",   // kept, must sort after the longer B
		"D": "abcdefghXY", // duplicate value: kept once
	})
	if len(vs) != 2 || vs[0] != "abcdefghXY" || vs[1] != "abcdefgh" {
		t.Fatalf("got %v", vs)
	}
}
