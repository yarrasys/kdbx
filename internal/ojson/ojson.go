// Package ojson provides a JSON object model that preserves key order, so that
// rewriting a committed .keepassxc.json produces a minimal diff (spec C1).
package ojson

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Object is a JSON object with stable key order. Values are either *Object
// (nested objects) or json.RawMessage (everything else, kept verbatim).
type Object struct {
	keys []string
	vals map[string]any
}

// New returns an empty Object.
func New() *Object {
	return &Object{vals: map[string]any{}}
}

// Parse decodes b into an order-preserving Object.
func Parse(b []byte) (*Object, error) {
	o := New()
	if err := json.Unmarshal(b, o); err != nil {
		return nil, err
	}
	return o, nil
}

// UnmarshalJSON decodes a JSON object, recording key order.
func (o *Object) UnmarshalJSON(b []byte) error {
	if o.vals == nil {
		o.vals = map[string]any{}
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return fmt.Errorf("ojson: expected a JSON object, got %v", tok)
	}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return err
		}
		key, ok := keyTok.(string)
		if !ok {
			return fmt.Errorf("ojson: non-string object key %v", keyTok)
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return err
		}
		var val any
		trimmed := bytes.TrimSpace(raw)
		if len(trimmed) > 0 && trimmed[0] == '{' {
			child := New()
			if err := child.UnmarshalJSON(trimmed); err != nil {
				return err
			}
			val = child
		} else {
			val = raw
		}
		if _, exists := o.vals[key]; !exists {
			o.keys = append(o.keys, key)
		}
		o.vals[key] = val
	}
	_, err = dec.Token() // consume '}'
	return err
}

// MarshalJSON re-emits the object in its recorded key order.
func (o *Object) MarshalJSON() ([]byte, error) {
	if o == nil {
		return []byte("null"), nil
	}
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, err := encodeString(k)
		if err != nil {
			return nil, err
		}
		buf.Write(kb)
		buf.WriteByte(':')
		switch v := o.vals[k].(type) {
		case *Object:
			vb, err := v.MarshalJSON()
			if err != nil {
				return nil, err
			}
			buf.Write(vb)
		case json.RawMessage:
			buf.Write(bytes.TrimSpace(v))
		default:
			return nil, fmt.Errorf("ojson: unsupported value type %T for key %q", v, k)
		}
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// encodeString marshals s as a JSON string without HTML escaping.
func encodeString(s string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// Keys returns the keys in file order.
func (o *Object) Keys() []string {
	if o == nil {
		return nil
	}
	out := make([]string, len(o.keys))
	copy(out, o.keys)
	return out
}

// Str returns the string value at key, or "" if absent or not a string.
func (o *Object) Str(key string) string {
	if o == nil {
		return ""
	}
	raw, ok := o.vals[key].(json.RawMessage)
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

// SetString sets key to val, updating in place if the key already exists.
func (o *Object) SetString(key, val string) {
	if o.vals == nil {
		o.vals = map[string]any{}
	}
	b, err := encodeString(val)
	if err != nil {
		return
	}
	if _, exists := o.vals[key]; !exists {
		o.keys = append(o.keys, key)
	}
	o.vals[key] = json.RawMessage(b)
}

// Obj returns the nested object at key, or nil if absent or not an object.
func (o *Object) Obj(key string) *Object {
	if o == nil {
		return nil
	}
	child, _ := o.vals[key].(*Object)
	return child
}

// EnsureObj returns the nested object at key, creating an empty one if needed.
func (o *Object) EnsureObj(key string) *Object {
	if o.vals == nil {
		o.vals = map[string]any{}
	}
	if child, ok := o.vals[key].(*Object); ok {
		return child
	}
	child := New()
	if _, exists := o.vals[key]; !exists {
		o.keys = append(o.keys, key)
	}
	o.vals[key] = child
	return child
}

// Delete removes key.
func (o *Object) Delete(key string) {
	if o == nil {
		return
	}
	if _, exists := o.vals[key]; !exists {
		return
	}
	delete(o.vals, key)
	for i, k := range o.keys {
		if k == key {
			o.keys = append(o.keys[:i], o.keys[i+1:]...)
			break
		}
	}
}

// Indent renders the object with 2-space indentation and a trailing newline,
// matching Python's json.dumps(indent=2) + "\n".
func (o *Object) Indent() ([]byte, error) {
	compact, err := o.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := json.Indent(&out, compact, "", "  "); err != nil {
		return nil, err
	}
	out.WriteByte('\n')
	return out.Bytes(), nil
}
