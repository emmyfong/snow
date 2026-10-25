package api

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Pane kinds, in PaneInfo.Kind.
const (
	PaneTerminal = "terminal" // a program in a pseudo-terminal; PaneUpdate carries its screen
	PaneChat     = "chat"     // a conversation with an agent; ChatSnapshot and ChatEvent carry it
)

// ChatSnapshot is the whole state of one chat pane. The server sends it
// when a client attaches, when a chat pane opens, and again to a client
// that fell behind; the client replaces what it had for the pane. Later
// ChatEvents change it through Apply.
type ChatSnapshot struct {
	Pane         int    `json:"pane"`
	Agent        string `json:"agent"`        // the agent the pane shows
	Conversation string `json:"conversation"` // the conversation's ID
	Title        string `json:"title,omitempty"`
	Model        string `json:"model"` // "provider/model" the next turn uses
	Effort       string `json:"effort,omitempty"`
	Mode         string `json:"mode"`  // the approval mode, for example "ask"
	State        string `json:"state"` // one of the ChatState* values
	Running      bool   `json:"running,omitempty"`
	Turn         int    `json:"turn,omitempty"` // the running or last turn
	// Earlier counts the oldest entries left out to keep the snapshot
	// inside MaxFrameSize.
	Earlier int         `json:"earlier,omitempty"`
	Entries []ChatEntry `json:"entries"`
	// Pending is the approval request the turn waits for, or nil.
	Pending *ChatApproval `json:"pending,omitempty"`
	Queue   []string      `json:"queue"`           // messages that wait for the running turn to end
	Usage   *ChatUsage    `json:"usage,omitempty"` // of the last reply
	// Cost is the estimated cost of the conversation's priced replies, in
	// US dollars.
	Cost float64 `json:"cost,omitempty"`
}

// Chat states, in ChatSnapshot.State and ChatEvent.State.
const (
	ChatStateIdle      = "idle"
	ChatStateStreaming = "streaming" // waiting for the model
	ChatStateTool      = "tool"      // running a tool
	ChatStateApproval  = "approval"  // waiting for the user's answer
)

// ChatEntry is one item of a chat transcript.
type ChatEntry struct {
	Kind string `json:"kind"` // one of the Entry* values
	Turn int    `json:"turn,omitempty"`
	// Text is the message for ChatEntryUser, the reply so far for
	// ChatEntryAssistant and ChatEntryThinking, the new model for ChatEntryModel, and
	// the model ID the vendor reported for ChatEntryReported.
	Text string `json:"text,omitempty"`
	// Requested is the "provider/model" a turn asked for, for
	// ChatEntryReported.
	Requested string `json:"requested,omitempty"`
	// Cut marks an ChatEntryAssistant reply that stopped before the model
	// finished it.
	Cut   bool       `json:"cut,omitempty"`
	Tool  *ChatTool  `json:"tool,omitempty"`  // for ChatEntryTool
	Error *ChatError `json:"error,omitempty"` // for ChatEntryError
}

// Kinds of ChatEntry.
const (
	ChatEntryUser        = "user"        // a message the user sent
	ChatEntryAssistant   = "assistant"   // the model's reply text
	ChatEntryThinking    = "thinking"    // the model's visible reasoning
	ChatEntryTool        = "tool"        // a tool call and, once it ran, its result
	ChatEntryModel       = "model"       // the model changed from the next turn on
	ChatEntryInterrupted = "interrupted" // the user stopped the turn
	ChatEntryError       = "error"       // a turn, or a change, failed
	// ChatEntryReported says the vendor reported another model than the one
	// the turn asked for.
	ChatEntryReported = "reported"
)

// ChatTool is one tool call and what came of it.
type ChatTool struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input,omitempty"`
	Diff  *ChatDiff       `json:"diff,omitempty"` // the change it asked to make
	// Done is set once the result arrived. Result is the result the model
	// got, cut to MaxToolResult bytes.
	Done    bool   `json:"done,omitempty"`
	Result  string `json:"result,omitempty"`
	IsError bool   `json:"isError,omitempty"`
}

// Bounds on what a client sends, so the messages and the queue a
// snapshot carries always fit a frame, even in JSON's longest escapes.
const (
	// MaxChatMessage bounds the bytes of a ChatInput's Text.
	MaxChatMessage = 16 << 10
	// MaxChatQueue bounds the messages that wait for a running turn; the
	// server refuses a message past it.
	MaxChatQueue = 4
)

// MaxToolResult bounds ChatTool.Result: a client shows a capped result, and
// a snapshot must fit in a frame.
const MaxToolResult = 16 << 10

// ChatApproval asks the user whether a tool call may run.
type ChatApproval struct {
	ID    string          `json:"id"` // the tool call's ID
	Tool  string          `json:"tool"`
	Input json.RawMessage `json:"input,omitempty"`
	Diff  *ChatDiff       `json:"diff,omitempty"`
	// Outside lists the paths outside the conversation folder that the
	// call names; they are why it asks, whatever the mode.
	Outside []string `json:"outside,omitempty"`
}

// ChatDiff is the change a tool call makes to one file.
type ChatDiff struct {
	Path string `json:"path"`
	Old  string `json:"old,omitempty"` // empty for a new file
	New  string `json:"new"`
}

// ChatUsage counts the tokens of one reply.
type ChatUsage struct {
	Requested  string  `json:"requested"`          // "provider/model" the turn asked for
	Reported   string  `json:"reported,omitempty"` // the model ID the vendor says it used
	Input      int     `json:"input"`
	Output     int     `json:"output"`
	CacheRead  int     `json:"cacheRead,omitempty"`
	CacheWrite int     `json:"cacheWrite,omitempty"`
	Cost       float64 `json:"cost,omitempty"` // in US dollars, estimated; valid when Priced
	Priced     bool    `json:"priced,omitempty"`
	// Window is the model's context window in tokens; 0 means unknown.
	Window int `json:"window,omitempty"`
}

// ChatError says why a turn or a change failed, and what the user can do.
type ChatError struct {
	Code    string `json:"code"` // one of the ChatErr* values
	Message string `json:"message"`
	// RetryAfter is how many seconds to wait before a retry, when the
	// provider says; 0 means it did not.
	RetryAfter float64  `json:"retryAfter,omitempty"`
	Choices    []string `json:"choices,omitempty"` // ChatChoice* values
}

// Chat error codes. They mirror the typed provider errors; a client shows
// an unknown code as ChatErrOther.
const (
	ChatErrSave           = "save" // the conversation file could not be written
	ChatErrNoKey          = "no-key"
	ChatErrBadKey         = "bad-key"
	ChatErrModelNotFound  = "model-not-found"
	ChatErrRateLimited    = "rate-limited"
	ChatErrOverloaded     = "overloaded"
	ChatErrContextTooLong = "context-too-long"
	ChatErrNetwork        = "network"
	ChatErrOther          = "other"
)

// Choices after a chat error. The server never takes one by itself.
const (
	ChatChoiceRetry     = "retry"      // send the turn again
	ChatChoicePickModel = "pick-model" // choose another model
	ChatChoiceWait      = "wait"       // wait for RetryAfter, then retry
	ChatChoiceLogin     = "login"      // save a key
)

// ChatEvent is one change to a chat pane, sent as the agent reports it.
// Kind says which fields are set. A receiver ignores a kind it does not
// know, so new kinds need no new ProtocolVersion.
type ChatEvent struct {
	Pane  int    `json:"pane"`
	Agent string `json:"agent"`
	Kind  string `json:"kind"` // one of the Event* values
	Turn  int    `json:"turn,omitempty"`
	// Text is the user's message for ChatEventTurnStarted (empty for a retry),
	// the new text for ChatEventText and ChatEventThinking, and the new title for
	// ChatEventTitle.
	Text     string        `json:"text,omitempty"`
	Reason   string        `json:"reason,omitempty"`   // for ChatEventTurnEnded: one of the End* values
	Tool     *ChatTool     `json:"tool,omitempty"`     // for ChatEventToolCall and ChatEventToolResult
	Approval *ChatApproval `json:"approval,omitempty"` // for ChatEventApproval
	State    string        `json:"state,omitempty"`    // for ChatEventState
	Queue    []string      `json:"queue,omitempty"`    // for ChatEventQueue; absent means empty
	Usage    *ChatUsage    `json:"usage,omitempty"`    // for ChatEventUsage
	Model    string        `json:"model,omitempty"`    // for ChatEventModel
	Effort   string        `json:"effort,omitempty"`   // for ChatEventModel
	Mode     string        `json:"mode,omitempty"`     // for ChatEventMode
	// Error is why ChatEventTurnEnded failed, or why ChatEventModel or ChatEventMode
	// could not be saved; the model or mode then stays as it was.
	Error *ChatError `json:"error,omitempty"`
}

// Kinds of ChatEvent.
const (
	ChatEventTurnStarted = "turn-started"
	ChatEventTurnEnded   = "turn-ended"
	ChatEventText        = "text"
	ChatEventThinking    = "thinking"
	ChatEventToolCall    = "tool-call"
	ChatEventToolResult  = "tool-result"
	ChatEventApproval    = "approval"
	ChatEventState       = "state"
	ChatEventQueue       = "queue"
	ChatEventUsage       = "usage"
	ChatEventModel       = "model"
	ChatEventMode        = "mode"
	ChatEventTitle       = "title"
)

// How a turn ends, in ChatEvent.Reason.
const (
	ChatEndDone        = "done"
	ChatEndInterrupted = "interrupted"
	ChatEndFailed      = "failed"
)

// ChatInput is what the user does in a chat pane.
type ChatInput struct {
	Pane  int    `json:"pane"`
	Agent string `json:"agent"`
	Kind  string `json:"kind"` // one of the Input* values
	// Text is the message for ChatInputMessage and the reason for a "no"
	// ChatInputAnswer.
	Text     string `json:"text,omitempty"`
	ID       string `json:"id,omitempty"`       // for ChatInputAnswer: the approval request's ID
	Decision string `json:"decision,omitempty"` // for ChatInputAnswer: one of the Decision* values
}

// Kinds of ChatInput.
const (
	ChatInputMessage   = "message"   // send Text; during a turn it waits in the queue
	ChatInputAnswer    = "answer"    // answer approval request ID
	ChatInputInterrupt = "interrupt" // stop the running turn
	ChatInputRetry     = "retry"     // run the last turn again after a failure
)

// Answers to an approval request.
const (
	ChatDecisionYes    = "yes"
	ChatDecisionAlways = "always" // yes to this tool for the rest of the conversation
	ChatDecisionNo     = "no"
)

// ErrInvalidChatInput is returned by ChatInput.Validate.
var ErrInvalidChatInput = errors.New("api: invalid chat input")

// Validate checks that in names a pane and an agent, and that its kind is
// known and has what that kind needs. Servers call it before acting on a
// ChatInput.
func (in *ChatInput) Validate() error {
	switch {
	case in.Pane <= 0:
		return fmt.Errorf("%w: no pane", ErrInvalidChatInput)
	case in.Agent == "":
		return fmt.Errorf("%w: no agent", ErrInvalidChatInput)
	}
	if len(in.Text) > MaxChatMessage {
		return fmt.Errorf("%w: text is over %d bytes", ErrInvalidChatInput, MaxChatMessage)
	}
	switch in.Kind {
	case ChatInputMessage:
		if in.Text == "" {
			return fmt.Errorf("%w: empty message", ErrInvalidChatInput)
		}
	case ChatInputAnswer:
		if in.ID == "" {
			return fmt.Errorf("%w: answer names no request", ErrInvalidChatInput)
		}
		switch in.Decision {
		case ChatDecisionYes, ChatDecisionAlways, ChatDecisionNo:
		default:
			return fmt.Errorf("%w: unknown decision %q", ErrInvalidChatInput, in.Decision)
		}
	case ChatInputInterrupt, ChatInputRetry:
	default:
		return fmt.Errorf("%w: unknown kind %q", ErrInvalidChatInput, in.Kind)
	}
	return nil
}

// MarshalJSON writes Entries and Queue as [] when they are empty.
func (s ChatSnapshot) MarshalJSON() ([]byte, error) {
	type wire ChatSnapshot
	if s.Entries == nil {
		s.Entries = []ChatEntry{}
	}
	if s.Queue == nil {
		s.Queue = []string{}
	}
	return json.Marshal(wire(s))
}

// Type implements Message.
func (*ChatSnapshot) Type() MsgType { return TypeChatSnapshot }
func (*ChatSnapshot) message()      {}

// Type implements Message.
func (*ChatEvent) Type() MsgType { return TypeChatEvent }
func (*ChatEvent) message()      {}

// Type implements Message.
func (*ChatInput) Type() MsgType { return TypeChatInput }
func (*ChatInput) message()      {}
