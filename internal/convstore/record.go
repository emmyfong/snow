package convstore

import (
	"encoding/json"
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
	TypeMode         Type = "mode"
	TypeInterruption Type = "interruption"
	TypePartial      Type = "partial"
	TypeRename       Type = "rename"
	TypeDamaged      Type = "damaged" // made by Load, never in a file
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
	Model  string     `json:"model,omitempty"`
	Blocks []Block    `json:"blocks"`
	Stop   StopReason `json:"stop,omitempty"`
	Usage  *Usage     `json:"usage,omitempty"`
	// CutOff is set by Load on a reply made from partial text whose whole
	// reply was never saved.
	CutOff bool `json:"cut_off,omitempty"`
}

// StopReason says why a reply ended. Load reads StopToolUse as a turn that
// goes on after the tool results, and any other reason as a turn that
// ended, so the agent loop maps each vendor reason to one of these.
type StopReason string

// The reasons a reply ends. They match provider.StopReason.
const (
	StopEndTurn   StopReason = "end_turn"
	StopToolUse   StopReason = "tool_use"
	StopMaxTokens StopReason = "max_tokens"
	StopRefusal   StopReason = "refusal"
	StopContext   StopReason = "context"
	StopOther     StopReason = "other"
)

// BlockKind says which part of a reply a Block is.
type BlockKind string

// The kinds of a Block.
const (
	BlockText             BlockKind = "text"
	BlockThinking         BlockKind = "thinking"
	BlockRedactedThinking BlockKind = "redacted_thinking"
	BlockToolUse          BlockKind = "tool_use"
)

// Block is one part of an assistant reply. Kind says which fields are set.
type Block struct {
	Kind BlockKind `json:"kind"`
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
	Decision  Decision  `json:"decision"`
	Reason    string    `json:"reason,omitempty"`
}

// Decision is the user's answer to an approval.
type Decision string

// The answers to an approval.
const (
	DecisionYes    Decision = "yes"    // run this call
	DecisionAlways Decision = "always" // run this and like calls in this conversation
	DecisionNo     Decision = "no"     // do not run it; Reason says why
)

// ModelSwitch records a change of model, which applies from the next turn.
type ModelSwitch struct {
	At     time.Time `json:"at"`
	Model  string    `json:"model"` // "provider/model"
	Effort string    `json:"effort,omitempty"`
}

// Mode records a change of the approval mode. The last one in the file
// is the conversation's mode; with none, the mode is the default.
type Mode struct {
	At   time.Time `json:"at"`
	Mode string    `json:"mode"` // for example "ask" or "auto"
}

// Interruption records a turn that stopped before it ended.
type Interruption struct {
	At     time.Time `json:"at"`
	Reason string    `json:"reason,omitempty"`
	// Failed is set when the turn ended in a failure, not at the user's
	// request, so a resumed view shows the error and not a stop.
	Failed bool `json:"failed,omitempty"`
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

// Damaged marks the place of a line that Load could not read. Load makes
// it; it is never in a file.
type Damaged struct {
	Line int
	Err  error
}

// Type implements Record.
func (*Damaged) Type() Type { return TypeDamaged }

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
func (*Mode) Type() Type { return TypeMode }

// Type implements Record.
func (*Interruption) Type() Type { return TypeInterruption }

// Type implements Record.
func (*Partial) Type() Type { return TypePartial }

// Type implements Record.
func (*Rename) Type() Type { return TypeRename }

// Type implements Record.
func (u *Unknown) Type() Type { return u.Kind }
