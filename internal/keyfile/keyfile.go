// Package keyfile mints and validates KeePass XML keyfiles (version 2.0), the
// sole credential for a kdbx vault (spec C4). Losing a keyfile makes its vault
// unrecoverable.
package keyfile

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"os"
	"strings"

	"github.com/yarrasys/kdbx/internal/kdbxerr"
	"github.com/yarrasys/kdbx/internal/secretio"
)

const keyLen = 32

// RenderXML produces the exact KeyFile v2 document the Python implementation
// writes: uppercase hex key data, plus a Hash attribute holding the uppercase
// hex of the first four bytes of SHA-256(key).
func RenderXML(key []byte) string {
	sum := sha256.Sum256(key)
	data := strings.ToUpper(hex.EncodeToString(key))
	checksum := strings.ToUpper(hex.EncodeToString(sum[:4]))
	return "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
		"<KeyFile>\n\t<Meta>\n\t\t<Version>2.0</Version>\n\t</Meta>\n" +
		fmt.Sprintf("\t<Key>\n\t\t<Data Hash=\"%s\">%s</Data>\n\t</Key>\n</KeyFile>\n", checksum, data)
}

// Mint writes a new random keyfile at path. It refuses to overwrite.
func Mint(path string) error {
	if _, err := os.Stat(path); err == nil {
		return kdbxerr.Preflight("keyfile already exists: %s", path)
	}
	key := make([]byte, keyLen)
	if _, err := rand.Read(key); err != nil {
		return kdbxerr.Wrap(err, "Runtime", 1, "generating key material")
	}
	return secretio.AtomicWriteSecret(path, []byte(RenderXML(key)))
}

type xmlKeyFile struct {
	XMLName xml.Name `xml:"KeyFile"`
	Meta    struct {
		Version string `xml:"Version"`
	} `xml:"Meta"`
	Key struct {
		Data struct {
			Hash  string `xml:"Hash,attr"`
			Value string `xml:",chardata"`
		} `xml:"Data"`
	} `xml:"Key"`
}

// Validate checks that path exists and is a self-consistent v2 keyfile. Any
// failure is a Locked error (exit 3) — the vault cannot be opened without it.
func Validate(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return kdbxerr.Wrap(err, "Locked", 3, "keyfile unreadable: %s", path)
	}
	var kf xmlKeyFile
	if err := xml.Unmarshal(b, &kf); err != nil {
		return kdbxerr.Wrap(err, "Locked", 3, "keyfile is not valid XML: %s", path)
	}
	if !strings.HasPrefix(kf.Meta.Version, "2") {
		return kdbxerr.Locked("unsupported keyfile version %q in %s", kf.Meta.Version, path)
	}
	raw := strings.Join(strings.Fields(kf.Key.Data.Value), "")
	key, err := hex.DecodeString(raw)
	if err != nil {
		return kdbxerr.Wrap(err, "Locked", 3, "keyfile data is not hex: %s", path)
	}
	sum := sha256.Sum256(key)
	if want := strings.ToUpper(hex.EncodeToString(sum[:4])); !strings.EqualFold(want, kf.Key.Data.Hash) {
		return kdbxerr.Locked("keyfile checksum mismatch: %s", path)
	}
	return nil
}
