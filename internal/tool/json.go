package tool

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// errTrailing is the cause of an error for input with data after its one
// JSON value.
var errTrailing = errors.New("data after the input object")

// jsonUnmarshal decodes raw into v. It refuses fields v does not have, so
// a typo in the model's input is an error it can read and fix, and data
// after the one value, so the approval step and the tool read the same
// input.
func jsonUnmarshal(raw []byte, v any) error { return decodeOne(raw, v, true) }

// decodeOne decodes the one JSON value in raw into v and refuses data
// after it. strict also refuses fields v does not have.
func decodeOne(raw []byte, v any, strict bool) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	if strict {
		d.DisallowUnknownFields()
	}
	if err := d.Decode(v); err != nil {
		return err
	}
	if _, err := d.Token(); !errors.Is(err, io.EOF) {
		return errTrailing
	}
	return nil
}

// schema returns a JSON Schema object with the given properties and
// required names.
func schema(props map[string]any, required ...string) json.RawMessage {
	s := map[string]any{"type": "object", "properties": props, "additionalProperties": false}
	if len(required) > 0 {
		s["required"] = required
	}
	b, err := json.Marshal(s)
	if err != nil {
		panic(err) // the schemas are literals in this package
	}
	return b
}

func prop(kind, desc string) map[string]any { return map[string]any{"type": kind, "description": desc} }
