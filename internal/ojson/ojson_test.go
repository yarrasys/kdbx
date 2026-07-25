package ojson

import (
	"strings"
	"testing"
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
