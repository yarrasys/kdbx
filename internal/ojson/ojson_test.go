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
	if !(iZed < iAlpha && iAlpha < iNew) {
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
