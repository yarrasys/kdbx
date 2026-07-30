package ojson

import (
	"strings"
	"testing"
	"time"
)

const sample = `{
  "project": "ideas",
  "defaultEnv": "dev",
  "envs": {
    "dev": {
      "vault": "~/v/dev.kdbx",
      "vars": {
        "ZED_KEY": "api/zed:password",
        "ALPHA_KEY": "api/alpha:password"
      }
    },
    "prod": {}
  }
}
`

func TestRoundTripPreservesKeyOrder(t *testing.T) {
	o, err := Parse([]byte(sample))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	got, err := o.Indent()
	if err != nil {
		t.Fatalf("Indent: %v", err)
	}
	if string(got) != sample {
		t.Fatalf("round-trip changed the file:\n--- got ---\n%s\n--- want ---\n%s", got, sample)
	}
}

func TestSetStringAppendsWithoutReordering(t *testing.T) {
	o, _ := Parse([]byte(sample))
	o.Obj("envs").Obj("dev").EnsureObj("vars").SetString("NEW_KEY", "api/new:password")
	got, _ := o.Indent()
	s := string(got)
	if !strings.Contains(s, `"NEW_KEY": "api/new:password"`) {
		t.Fatalf("new var missing:\n%s", s)
	}
	iZed := strings.Index(s, "ZED_KEY")
	iAlpha := strings.Index(s, "ALPHA_KEY")
	iNew := strings.Index(s, "NEW_KEY")
	if iZed >= iAlpha || iAlpha >= iNew {
		t.Fatalf("order not preserved (ZED then ALPHA then NEW):\n%s", s)
	}
	if !strings.HasPrefix(s, "{\n  \"project\": \"ideas\",") {
		t.Fatalf("top-level key order changed — project must remain first:\n%s", s)
	}
}

func TestSetStringOverwritesInPlace(t *testing.T) {
	o, _ := Parse([]byte(sample))
	o.Obj("envs").Obj("dev").Obj("vars").SetString("ZED_KEY", "api/zed2:password")
	got, _ := o.Indent()
	s := string(got)
	if !strings.Contains(s, `"ZED_KEY": "api/zed2:password"`) {
		t.Fatalf("value not updated:\n%s", s)
	}
	if strings.Index(s, "ZED_KEY") > strings.Index(s, "ALPHA_KEY") {
		t.Fatal("overwriting must not move the key to the end")
	}
}

func TestEnsureObjCreatesMissingLevels(t *testing.T) {
	o, _ := Parse([]byte(`{"envs":{}}`))
	o.Obj("envs").EnsureObj("staging").EnsureObj("vars").SetString("K", "p:password")
	got, _ := o.Indent()
	if !strings.Contains(string(got), `"staging"`) || !strings.Contains(string(got), `"K": "p:password"`) {
		t.Fatalf("nested creation failed:\n%s", got)
	}
}

func TestObjReturnsNilForMissingOrNonObject(t *testing.T) {
	o, _ := Parse([]byte(`{"a": "string"}`))
	if o.Obj("a") != nil {
		t.Fatal("string value must not be returned as an object")
	}
	if o.Obj("missing") != nil {
		t.Fatal("missing key must return nil")
	}
}

func TestSetStringEscapesNonASCIILikePython(t *testing.T) {
	// Python's json.dumps defaults to ensure_ascii=True, and every existing
	// .keepassxc.json was written by it. Emitting raw UTF-8 here would make the
	// two implementations produce different bytes for the same committed file.
	o, _ := Parse([]byte(`{"vars":{}}`))
	o.Obj("vars").SetString("K", "café/naïve")
	got, err := o.Indent()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `"caf\u00e9/na\u00efve"`) {
		t.Fatalf("non-ASCII not escaped as Python would write it:\n%s", got)
	}
}

func TestSetStringEscapesAstralPlaneAsSurrogatePair(t *testing.T) {
	o, _ := Parse([]byte(`{"vars":{}}`))
	o.Obj("vars").SetString("K", "\U0001F511") // 🔑
	got, err := o.Indent()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `"\ud83d\udd11"`) {
		t.Fatalf("astral-plane rune not encoded as a surrogate pair:\n%s", got)
	}
}

func TestPreEscapedValuesRoundTripByteExactly(t *testing.T) {
	// A file already written by Python must survive untouched.
	src := "{\n  \"vars\": {\n    \"K\": \"caf\\u00e9\"\n  }\n}\n"
	o, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got, err := o.Indent()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != src {
		t.Fatalf("round-trip altered a pre-escaped file:\ngot  %q\nwant %q", got, src)
	}
}

// TestParseRejectsDeeplyNestedObjects pins a depth limit on nested objects.
//
// Without one, parsing is quadratic in nesting depth: each level captures the
// whole remaining subtree as a json.RawMessage and then re-scans it, so the input
// is re-parsed once per level. Measured before the limit existed: 1 ms at depth
// 100, 24 ms at 1,000, and 1.5 s at 10,000.
//
// It matters because pointer discovery walks *up* from the working directory, so
// kdbx will read a `.keepassxc.json` it did not put there — checking out a hostile
// repository and running any pointer-resolving command (`envs`, `check`, `list`,
// `run`) is enough to reach this. A real pointer nests three levels
// (root -> envs -> <env>), so the limit costs legitimate files nothing.
func TestParseRejectsDeeplyNestedObjects(t *testing.T) {
	const depth = 5000
	deep := []byte(strings.Repeat(`{"a":`, depth) + `1` + strings.Repeat(`}`, depth))

	start := time.Now()
	_, err := Parse(deep)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("Parse accepted a %d-deep object; want a depth-limit error", depth)
	}
	if !strings.Contains(err.Error(), "nested too deeply") {
		t.Errorf("error should name the cause, got: %v", err)
	}
	// Generous, but a return to quadratic scanning blows straight past it.
	if elapsed > 2*time.Second {
		t.Errorf("rejecting a %d-deep object took %s; the depth limit is not short-circuiting",
			depth, elapsed)
	}
}

// TestParseAcceptsSchemaDepth guards the other side of the limit: the real
// pointer schema, and comfortable headroom above it, must still parse.
func TestParseAcceptsSchemaDepth(t *testing.T) {
	if _, err := Parse([]byte(sample)); err != nil {
		t.Fatalf("the real pointer schema must parse: %v", err)
	}
	const ok = 16
	nested := []byte(strings.Repeat(`{"a":`, ok) + `1` + strings.Repeat(`}`, ok))
	if _, err := Parse(nested); err != nil {
		t.Fatalf("%d levels is within the limit but was rejected: %v", ok, err)
	}
}
