package keyfile

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderXMLMatchesPythonByteForByte(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	sum := sha256.Sum256(key)
	want := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
		"<KeyFile>\n\t<Meta>\n\t\t<Version>2.0</Version>\n\t</Meta>\n" +
		"\t<Key>\n\t\t<Data Hash=\"" + strings.ToUpper(hex.EncodeToString(sum[:4])) + "\">" +
		strings.ToUpper(hex.EncodeToString(key)) + "</Data>\n\t</Key>\n</KeyFile>\n"

	if got := RenderXML(key); got != want {
		t.Fatalf("keyfile XML drifted from the Python format:\n--- got ---\n%q\n--- want ---\n%q", got, want)
	}
}

func TestMintCreatesValidOwnerOnlyKeyfile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "dev.keyx")
	if err := Mint(p); err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if err := Validate(p); err != nil {
		t.Fatalf("Validate on freshly minted keyfile: %v", err)
	}
	b, _ := os.ReadFile(p)
	if !strings.Contains(string(b), "<Version>2.0</Version>") {
		t.Fatalf("not a v2 keyfile:\n%s", b)
	}
}

func TestMintIsUnpredictable(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.keyx")
	b := filepath.Join(dir, "b.keyx")
	if err := Mint(a); err != nil {
		t.Fatal(err)
	}
	if err := Mint(b); err != nil {
		t.Fatal(err)
	}
	ab, _ := os.ReadFile(a)
	bb, _ := os.ReadFile(b)
	if string(ab) == string(bb) {
		t.Fatal("two minted keyfiles are identical — the key is not random")
	}
}

func TestMintRefusesToOverwrite(t *testing.T) {
	p := filepath.Join(t.TempDir(), "dev.keyx")
	if err := Mint(p); err != nil {
		t.Fatal(err)
	}
	if err := Mint(p); err == nil {
		t.Fatal("Mint must refuse to overwrite an existing keyfile")
	}
}

func TestValidateRejectsCorruptHash(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bad.keyx")
	body := RenderXML(make([]byte, 32))
	body = strings.Replace(body, "Hash=\"", "Hash=\"AA", 1)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Validate(p); err == nil {
		t.Fatal("Validate must reject a keyfile whose hash does not match")
	}
}

func TestValidateMissingFileIsLockedError(t *testing.T) {
	if err := Validate(filepath.Join(t.TempDir(), "absent.keyx")); err == nil {
		t.Fatal("Validate must fail for a missing keyfile")
	}
}
