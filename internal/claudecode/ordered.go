package claudecode

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// member is one key of a JSON object, with its value's bytes exactly as
// read, so values Snow does not change are written back as they were.
type member struct {
	key   string
	value json.RawMessage
}

// object is a JSON object that keeps its keys' order.
type object []member

// errNotObject and errNotArray say a value has another JSON type.
var (
	errNotObject = errors.New("not a JSON object")
	errNotArray  = errors.New("not a JSON array")
)

// parseObject reads raw as an object.
func parseObject(raw []byte) (object, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := expectDelim(dec, '{', errNotObject); err != nil {
		return nil, err
	}
	var out object
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, errNotObject
		}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		out = append(out, member{key: key, value: v})
	}
	return out, expectEnd(dec, '}')
}

// parseArray reads raw as an array, each item's bytes as read.
func parseArray(raw []byte) ([]json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := expectDelim(dec, '[', errNotArray); err != nil {
		return nil, err
	}
	var out []json.RawMessage
	for dec.More() {
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, expectEnd(dec, ']')
}

func expectDelim(dec *json.Decoder, d json.Delim, wrong error) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if tok != d {
		return wrong
	}
	return nil
}

// expectEnd reads the closing delimiter and checks nothing follows.
func expectEnd(dec *json.Decoder, d json.Delim) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if tok != d {
		return fmt.Errorf("want %q, got %v", d, tok)
	}
	if _, err := dec.Token(); err == nil {
		return errors.New("more JSON after the value")
	}
	return nil
}

// get returns the value of key.
func (o object) get(key string) (json.RawMessage, bool) {
	for _, m := range o {
		if m.key == key {
			return m.value, true
		}
	}
	return nil, false
}

// set replaces key's value in place, or adds key at the end.
func (o object) set(key string, v json.RawMessage) object {
	for i, m := range o {
		if m.key == key {
			o[i].value = v
			return o
		}
	}
	return append(o, member{key: key, value: v})
}

// without returns o without key.
func (o object) without(key string) object {
	out := o[:0:0]
	for _, m := range o {
		if m.key != key {
			out = append(out, m)
		}
	}
	return out
}

// encode writes o compactly; indent formats the whole file once at the end.
func (o object) encode() json.RawMessage {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, m := range o {
		if i > 0 {
			b.WriteByte(',')
		}
		b.Write(quote(m.key))
		b.WriteByte(':')
		b.Write(m.value)
	}
	b.WriteByte('}')
	return b.Bytes()
}

func encodeArray(items []json.RawMessage) json.RawMessage {
	b := []byte{'['}
	for i, it := range items {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, it...)
	}
	return append(b, ']')
}

// quote encodes s as a JSON string, leaving <, >, and & as they are, as
// Claude Code writes them.
func quote(s string) []byte {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s) // a string always encodes
	return bytes.TrimRight(b.Bytes(), "\n")
}

// format is how a settings file is laid out, so Snow writes it back the
// same way: its indent, line endings, byte order mark, and last newline.
type format struct {
	indent  string
	crlf    bool
	bom     bool
	finalNL bool
}

var byteOrderMark = []byte("\xEF\xBB\xBF")

// detectFormat returns data's format and data without a byte order mark.
// A file with no indented line, empty or on one line, gets Claude Code's
// own: two spaces and a newline at the end.
func detectFormat(data []byte) (format, []byte) {
	f := format{indent: "  ", finalNL: true}
	if bytes.HasPrefix(data, byteOrderMark) {
		f.bom, data = true, data[len(byteOrderMark):]
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return f, data
	}
	f.crlf = bytes.Contains(data, []byte("\r\n"))
	f.finalNL = bytes.HasSuffix(data, []byte("\n"))
	for line := range bytes.Lines(data) {
		// The first indented line is one level deep.
		if rest := bytes.TrimLeft(line, " \t"); len(rest) < len(line) && len(bytes.TrimSpace(rest)) > 0 {
			f.indent = string(line[:len(line)-len(rest)])
			break
		}
	}
	return f, data
}

// indentJSON formats raw in format f. Values that were formatted the same way
// come out byte for byte.
func (f format) indentJSON(raw []byte) ([]byte, error) {
	var b bytes.Buffer
	if f.bom {
		b.Write(byteOrderMark)
	}
	if err := json.Indent(&b, raw, "", f.indent); err != nil {
		return nil, err
	}
	if f.finalNL {
		b.WriteByte('\n')
	}
	out := b.Bytes()
	if f.crlf {
		// JSON strings hold no raw newline, so every newline is layout.
		out = bytes.ReplaceAll(out, []byte("\n"), []byte("\r\n"))
	}
	return out, nil
}
