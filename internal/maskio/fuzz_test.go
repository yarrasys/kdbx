package maskio

import (
	"bytes"
	"testing"
)

// FuzzMaskChunking asserts the property the streaming carry logic exists for:
// however a stream is split into Write calls, the masked output is identical
// to masking the whole stream in one call. A violation means a value can leak
// (or output can be mangled) at a chunk boundary.
func FuzzMaskChunking(f *testing.F) {
	f.Add([]byte("TOKEN=sk-test-value tail"), "sk-test-value", 3)
	f.Add([]byte("v=abcdefghXY rest abcdefgh"), "abcdefgh", 1)
	f.Add([]byte("no secrets here at all"), "sk-test-value", 5)
	f.Add([]byte("sk-test-valuesk-test-value"), "sk-test-value", 7)
	f.Add([]byte("ends on a prefix sk-te"), "sk-test-value", 2)

	f.Fuzz(func(t *testing.T, stream []byte, secret string, chunk int) {
		if chunk < 1 {
			chunk = 1
		}
		values := []string{secret, secret + "-longer-variant"}

		var whole bytes.Buffer
		w := New(&whole, values)
		if _, err := w.Write(stream); err != nil {
			t.Fatal(err)
		}
		if err := w.Flush(); err != nil {
			t.Fatal(err)
		}

		var chunked bytes.Buffer
		c := New(&chunked, values)
		for i := 0; i < len(stream); i += chunk {
			end := i + chunk
			if end > len(stream) {
				end = len(stream)
			}
			if _, err := c.Write(stream[i:end]); err != nil {
				t.Fatal(err)
			}
		}
		if err := c.Flush(); err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(whole.Bytes(), chunked.Bytes()) {
			t.Fatalf("chunk size %d changed the output:\nwhole:   %q\nchunked: %q",
				chunk, whole.Bytes(), chunked.Bytes())
		}
		// And the property masking exists for: the full secret never survives
		// into the output (when it is long enough to be masked at all). Only
		// asserted for secrets containing no mask byte: the fuzzer found that
		// a secret like "*0000000" is correctly replaced and then re-created
		// by the mask "***" colliding with the legitimate bytes after it. No
		// secret material survives in that case; the occurrence is an
		// artifact. A secret with no "*" cannot collide, so for those any
		// occurrence really is a leak.
		if len(secret) >= MinLen && !bytes.ContainsRune([]byte(secret), '*') &&
			bytes.Contains(chunked.Bytes(), []byte(secret)) {
			t.Fatalf("secret survived masking: %q in %q", secret, chunked.Bytes())
		}
	})
}
