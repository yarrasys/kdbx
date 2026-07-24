// Package jsonout renders the --json output envelopes (spec N1). It never
// carries a secret value.
package jsonout

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/yarrasys/kdbx/internal/kdbxerr"
)

// Write emits v as a single compact JSON line.
func Write(w io.Writer, v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return err
	}
	_, err := w.Write(buf.Bytes())
	return err
}

type errEnvelope struct {
	Error errBody `json:"error"`
}

type errBody struct {
	Op   string `json:"op"`
	Exit int    `json:"exit"`
	Kind string `json:"kind"`
}

// WriteError emits the failure envelope for op.
func WriteError(w io.Writer, op string, err error) error {
	return Write(w, errEnvelope{Error: errBody{
		Op:   op,
		Exit: kdbxerr.CodeOf(err),
		Kind: kdbxerr.KindOf(err),
	}})
}
