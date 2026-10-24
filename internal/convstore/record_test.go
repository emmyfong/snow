package convstore

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

// everyRecord holds one record of each type this package knows.
var everyRecord = []Record{
	&Meta{Version: Version, ID: testID, Title: "t", Folder: "/home/u/p", Profile: "bash", Created: at, Model: "anthropic/claude-opus-5-5", Effort: "high"},
	&User{At: at, Text: "Hello\nthere \"quoted\""},
	&Assistant{At: at, Model: "claude-opus-5-5", Stop: "tool_use", Usage: &Usage{Input: 10, Output: 5, CacheRead: 2}, Blocks: []Block{
		{Kind: BlockThinking, Text: "hmm", Signature: "sig"},
		{Kind: BlockRedactedThinking, Data: "opaque"},
		{Kind: BlockText, Text: "Let me look."},
		{Kind: BlockToolUse, ID: "call_1", Name: "read", Input: json.RawMessage(`{"path":"a.go"}`)},
	}},
	&ToolResult{At: at, ToolUseID: "call_1", Content: "package a", IsError: true},
	&Approval{At: at, ToolUseID: "call_1", Decision: "no", Reason: "not that file"},
	&ModelSwitch{At: at, Model: "openai/gpt-6.1-sol", Effort: "low"},
	&Mode{At: at, Mode: "auto"},
	&Interruption{At: at, Reason: "user"},
	&Partial{At: at, Text: "Half a rep"},
	&Rename{At: at, Title: "New title"},
}

func TestEncodeDecode(t *testing.T) {
	for _, r := range everyRecord {
		line, err := Encode(r)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Count(line, []byte("\n")) != 1 || line[len(line)-1] != '\n' {
			t.Fatalf("%s: not one line: %q", r.Type(), line)
		}
		if want := `{"type":"` + string(r.Type()) + `",`; !bytes.HasPrefix(line, []byte(want)) {
			t.Errorf("%s: line starts %q, want %q", r.Type(), line[:min(len(line), 30)], want)
		}
		got, err := Decode(line)
		if err != nil {
			t.Fatalf("%s: %v", r.Type(), err)
		}
		if !reflect.DeepEqual(got, r) {
			t.Errorf("%s: round trip\n got %#v\nwant %#v", r.Type(), got, r)
		}
	}
}

// TestDecodeUnknown: a type from a later version is kept whole, and is
// written back unchanged.
func TestDecodeUnknown(t *testing.T) {
	line := []byte(`{"type":"question","at":"2026-10-23T19:15:00Z","text":"Which file?"}`)
	r, err := Decode(append(line, '\r', '\n'))
	if err != nil {
		t.Fatal(err)
	}
	u, ok := r.(*Unknown)
	if !ok || u.Type() != "question" || !bytes.Equal(u.Raw, line) {
		t.Fatalf("got %#v", r)
	}
	back, err := Encode(u)
	if err != nil || !bytes.Equal(back, append(line, '\n')) {
		t.Fatalf("encode back: %q, %v", back, err)
	}
}

// TestEncodeBad: a record that would not be one valid line is refused.
func TestEncodeBad(t *testing.T) {
	for _, r := range []Record{nil, (*User)(nil), &Damaged{Line: 1},
		&Unknown{Kind: "x", Raw: []byte("{\"type\":\"x\"}\n{}")}, &Unknown{Kind: "x", Raw: []byte("{")}} {
		if line, err := Encode(r); err == nil {
			t.Errorf("Encode(%#v) = %q", r, line)
		}
	}
}

func TestDecodeBad(t *testing.T) {
	for _, line := range []string{``, `{`, `[]`, `{"text":"no type"}`, `{"type":"user","text":5}`} {
		if r, err := Decode([]byte(line)); err == nil {
			t.Errorf("%q decoded to %#v", line, r)
		}
	}
	if _, err := Decode([]byte(`{}`)); !errors.Is(err, errEmptyType) {
		t.Errorf("no type: %v", err)
	}
}
