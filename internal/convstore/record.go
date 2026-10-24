// Package convstore keeps conversations on disk so that nothing the user and
// the agent said is lost: one append-only JSON Lines file per conversation,
// one record per line, each written whole and flushed before Append returns.
package convstore

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Version is the file format version that this package writes in the meta
// record. A file with a newer version is not read.
const Version = 1

// Type names the kind of a record. It is the "type" field of each line.
type Type string

// The record types this package knows. A reader keeps a line of any other
// type as *Unknown, so later versions can add types.
const (
	TypeMeta         Type = "meta"
	TypeUser         Type = "user"
	TypeAssistant    Type = "assistant"
	TypeToolResult   Type = "tool_result"
	TypeApproval     Type = "approval"
	TypeModelSwitch  Type = "model_switch"
	TypeInterruption Type = "interruption"
	TypePartial      Type = "partial"
	TypeRename       Type = "rename"
)

// Record is one line of a conversation file.
type Record interface{ Type() Type }

// Meta is the first record of every file.
type Meta struct {
	Version int       `json:"version"`
	ID      string    `json:"id"`
	Title   string    `json:"title,omitempty"`
	Folder  string    `json:"folder"`
	Profile string    `json:"profile,omitempty"`
	Created time.Time `json:"created"`
	Model   string    `json:"model"` // "provider/model"
	Effort  string    `json:"effort,omitempty"`
}

// User is a message the user sent.
type User struct {
	At   time.Time `json:"at"`
	Text string    `json:"text"`
}

// Assistant is one whole reply of the model. Its blocks keep the order the
// model sent them in: vendors accept reasoning and tool calls back only in
// that order.
type Assistant struct {
	At time.Time `json:"at"`
	// Model is the ID the vendor reported, which can differ from the one
	// requested.
	Model  string  `json:"model,omitempty"`
	Blocks []Block `json:"blocks"`
	Stop   string  `json:"stop,omitempty"`
	Usage  *Usage  `json:"usage,omitempty"`
	// CutOff is set by Load on a reply made from partial text whose whole
	// reply was never saved.
	CutOff bool `json:"cut_off,omitempty"`
}

// StopToolUse is the Stop of a reply that waits for tool results; the turn
// goes on after them.
const StopToolUse = "tool_use"

// The kinds of a Block.
const (
	BlockText             = "text"
	BlockThinking         = "thinking"
	BlockRedactedThinking = "redacted_thinking"
	BlockToolUse          = "tool_use"
)

// Block is one part of an assistant reply. Kind says which fields are set.
type Block struct {
	Kind string `json:"kind"`
	// Text is the text of a text or thinking block.
	Text string `json:"text,omitempty"`
	// Signature is the vendor's proof for a thinking block; Data is the
	// vendor's opaque redacted reasoning. Both go back unchanged.
	Signature string `json:"signature,omitempty"`
	Data      string `json:"data,omitempty"`
	// ID, Name, and Input describe a tool call.
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// Usage counts the tokens of a reply.
type Usage struct {
	Input      int `json:"input"`
	Output     int `json:"output"`
	CacheRead  int `json:"cache_read,omitempty"`
	CacheWrite int `json:"cache_write,omitempty"`
}

// ToolResult is the result of one tool call.
type ToolResult struct {
	At        time.Time `json:"at"`
	ToolUseID string    `json:"tool_use_id"`
	Content   string    `json:"content"`
	IsError   bool      `json:"is_error,omitempty"`
	// Lost is set by Load on a stand-in result for a tool call whose
	// result was never saved, so the next request to the provider is valid.
	Lost bool `json:"lost,omitempty"`
}

// Approval records the user's answer to a tool call that needed approval.
type Approval struct {
	At        time.Time `json:"at"`
	ToolUseID string    `json:"tool_use_id"`
	Decision  string    `json:"decision"` // for example "yes", "always", "no"
	Reason    string    `json:"reason,omitempty"`
}

// ModelSwitch records a change of model, which applies from the next turn.
type ModelSwitch struct {
	At     time.Time `json:"at"`
	Model  string    `json:"model"` // "provider/model"
	Effort string    `json:"effort,omitempty"`
}

// Interruption records a turn that stopped before it ended.
type Interruption struct {
	At     time.Time `json:"at"`
	Reason string    `json:"reason,omitempty"`
}

// Partial is the text of a reply that is still streaming. A later Partial
// replaces an earlier one; an Assistant record replaces them all.
type Partial struct {
	At   time.Time `json:"at"`
	Text string    `json:"text"`
}

// Rename records a new title for the conversation.
type Rename struct {
	At    time.Time `json:"at"`
	Title string    `json:"title"`
}

// Unknown is a record whose type this version does not know. Raw is the
// whole line, kept unchanged.
type Unknown struct {
	Kind Type
	Raw  json.RawMessage
}

// Type implements Record.
func (*Meta) Type() Type { return TypeMeta }

// Type implements Record.
func (*User) Type() Type { return TypeUser }

// Type implements Record.
func (*Assistant) Type() Type { return TypeAssistant }

// Type implements Record.
func (*ToolResult) Type() Type { return TypeToolResult }

// Type implements Record.
func (*Approval) Type() Type { return TypeApproval }

// Type implements Record.
func (*ModelSwitch) Type() Type { return TypeModelSwitch }

// Type implements Record.
func (*Interruption) Type() Type { return TypeInterruption }

// Type implements Record.
func (*Partial) Type() Type { return TypePartial }

// Type implements Record.
func (*Rename) Type() Type { return TypeRename }

// Type implements Record.
func (u *Unknown) Type() Type { return u.Kind }

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
	case TypeInterruption:
		return &Interruption{}
	case TypePartial:
		return &Partial{}
	case TypeRename:
		return &Rename{}
	}
	return nil
}

// errEmptyType is the cause of a decode error for a line with no type.
var errEmptyType = errors.New("record has no type")

// Encode returns r as one JSON line, ending in a newline. The line starts
// with the record's type.
func Encode(r Record) ([]byte, error) {
	switch r := r.(type) {
	case *Unknown:
		return append(bytes.Clone(r.Raw), '\n'), nil
	case *Damaged:
		return nil, errors.New("encode damaged record: it exists only after a load")
	}
	body, err := json.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("encode %s record: %w", r.Type(), err)
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
