package tool

import (
	"bytes"
	"encoding/json"
)

// jsonUnmarshal decodes raw into v and refuses fields v does not have, so
// a typo in the model's input is an error it can read and fix.
func jsonUnmarshal(raw []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	return d.Decode(v)
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
