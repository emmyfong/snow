package convstore

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// newRecord returns an empty record of type t, or nil for a type this
// version does not know.
func newRecord(t Type) Record {
	switch t {
	case TypeMeta:
		return &Meta{}
	case TypeUser:
		return &User{}
	case TypeAssistant:
		return &Assistant{}
	case TypeToolResult:
		return &ToolResult{}
	case TypeApproval:
		return &Approval{}
	case TypeModelSwitch:
		return &ModelSwitch{}
	case TypeMode:
		return &Mode{}
	case TypeInterruption:
		return &Interruption{}
	case TypePartial:
		return &Partial{}
	case TypeRename:
		return &Rename{}
	case TypeQuestion:
		return &Question{}
	case TypeAnswer:
		return &Answer{}
	}
	return nil
}

// errEmptyType is the cause of a decode error for a line with no type.
var errEmptyType = errors.New("record has no type")

// Encode returns r as one JSON line, ending in a newline. The line starts
// with the record's type.
func Encode(r Record) ([]byte, error) {
	switch r := r.(type) {
	case nil:
		return nil, errors.New("encode record: no record")
	case *Unknown:
		// Raw goes out as it is, so it must still be one JSON line.
		if bytes.ContainsAny(r.Raw, "\r\n") || !json.Valid(r.Raw) {
			return nil, fmt.Errorf("encode %s record: raw data is not one JSON line", r.Kind)
		}
		return append(bytes.Clone(r.Raw), '\n'), nil
	case *Damaged:
		return nil, errors.New("encode damaged record: it exists only after a load")
	}
	body, err := json.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("encode %s record: %w", r.Type(), err)
	}
	if body[0] != '{' {
		// A nil pointer of a record type marshals as null.
		return nil, fmt.Errorf("encode %s record: no record", r.Type())
	}
	kind, err := json.Marshal(r.Type())
	if err != nil {
		return nil, fmt.Errorf("encode %s record: %w", r.Type(), err)
	}
	line := make([]byte, 0, len(body)+len(kind)+12)
	line = append(line, `{"type":`...)
	line = append(line, kind...)
	if len(body) > 2 {
		line = append(line, ',')
		line = append(line, body[1:]...)
	} else {
		line = append(line, '}')
	}
	return append(line, '\n'), nil
}

// Decode reads one line, with or without its newline. A type this version
// does not know gives *Unknown.
func Decode(line []byte) (Record, error) {
	line = bytes.TrimRight(line, "\r\n")
	var head struct {
		Type Type `json:"type"`
	}
	if err := json.Unmarshal(line, &head); err != nil {
		return nil, fmt.Errorf("decode record: %w", err)
	}
	if head.Type == "" {
		return nil, fmt.Errorf("decode record: %w", errEmptyType)
	}
	r := newRecord(head.Type)
	if r == nil {
		return &Unknown{Kind: head.Type, Raw: bytes.Clone(line)}, nil
	}
	if err := json.Unmarshal(line, r); err != nil {
		return nil, fmt.Errorf("decode %s record: %w", head.Type, err)
	}
	return r, nil
}
