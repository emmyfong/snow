package provider

import "encoding/json"

// Event is one piece of a streamed reply: *Start, *TextDelta,
// *ThinkingDelta, *ToolUse, *Usage, or *Stop.
type Event interface{ event() }

// Block is one piece of a Message: *Text, *Thinking, *ToolUse, or
// *ToolResult.
type Block interface{ block() }

// Start opens a reply. Model is the ID the vendor says it used, which can
// differ from the one requested.
type Start struct{ Model string }

// TextDelta is the next part of the reply's text.
type TextDelta struct{ Text string }

// ThinkingDelta is the next part of the model's visible reasoning.
type ThinkingDelta struct{ Text string }

// ToolUse is a call the model makes to a tool, as an event when it arrives
// and as a block when the conversation is sent back.
type ToolUse struct {
	ID    string
	Name  string
	Input json.RawMessage
}

// Usage counts the tokens of a reply so far. A later Usage replaces an
// earlier one.
type Usage struct {
	Input      int
	Output     int
	CacheRead  int
	CacheWrite int
}

// StopReason says why a reply ended.
type StopReason string

// The reasons a reply ends.
const (
	StopEndTurn   StopReason = "end_turn"   // the model finished
	StopToolUse   StopReason = "tool_use"   // the model waits for tool results
	StopMaxTokens StopReason = "max_tokens" // the reply hit its length limit
	StopRefusal   StopReason = "refusal"    // the model declined
	StopOther     StopReason = "other"      // a reason Snow does not know
)

// Stop ends a reply.
type Stop struct{ Reason StopReason }

// Text is text in a Message.
type Text struct{ Text string }

// Thinking is reasoning from an earlier reply, sent back unchanged.
// Signature is the vendor's proof that the text is its own.
type Thinking struct {
	Text      string
	Signature string
}

// ToolResult answers a ToolUse.
type ToolResult struct {
	ToolUseID string
	Content   string
	IsError   bool
}

func (*Start) event()         {}
func (*TextDelta) event()     {}
func (*ThinkingDelta) event() {}
func (*ToolUse) event()       {}
func (*Usage) event()         {}
func (*Stop) event()          {}

func (*Text) block()       {}
func (*Thinking) block()   {}
func (*ToolUse) block()    {}
func (*ToolResult) block() {}
