package agent

import (
	"encoding/json"
	"time"
)

// ID names one agent for the life of the agent. The main agent of a
// conversation has the conversation's ID, so it stays the same across a
// resume; later, subagents get IDs of their own.
type ID string

// Event is something an agent did, in one shape for every kind: who, in
// which conversation, when, and a Body that says what.
type Event struct {
	Agent        ID
	Conversation string
	At           time.Time
	Body         Body
}

// Body is what happened: *TurnStarted, *TurnEnded, *TextDelta,
// *ThinkingDelta, *ToolCall, *ToolResult, *ApprovalRequest, *Question,
// *QuestionAnswered, *TasksChanged, *Usage, *StateChanged, *QueueChanged,
// *ModelChanged, *ModeChanged, or *TitleChanged.
type Body interface{ body() }

// TurnStarted opens a turn. Turns count from 1 in each run of the
// conversation. Text is the user's message that starts it; a retry that
// sends no new message has none.
type TurnStarted struct {
	Turn int
	Text string
}

// EndReason says how a turn ended.
type EndReason string

// The ways a turn ends.
const (
	EndDone        EndReason = "done"        // the model finished
	EndInterrupted EndReason = "interrupted" // the user stopped it
	EndFailed      EndReason = "failed"      // see TurnEnded.Failure
)

// TurnEnded closes a turn. Failure is set when Reason is EndFailed.
type TurnEnded struct {
	Turn    int
	Reason  EndReason
	Failure *Failure
}

// TextDelta is the next part of the reply's text.
type TextDelta struct{ Text string }

// ThinkingDelta is the next part of the model's visible reasoning.
type ThinkingDelta struct{ Text string }

// ToolCall is a call the model made. It comes before the call runs, and
// before any approval.
type ToolCall struct {
	ID    string
	Name  string
	Input json.RawMessage
}

// ToolResult is the result of a call, as the model gets it.
type ToolResult struct {
	ID      string
	Content string
	IsError bool
}

// ApprovalRequest asks the user whether a tool call may run. The turn
// waits for the answer.
type ApprovalRequest struct {
	ID    string // the tool call's ID
	Tool  string
	Input json.RawMessage
	Diff  *Diff // for a tool that changes a file; nil otherwise
	// Outside lists the paths outside the conversation folder that the
	// call names; they are why it asks, whatever the mode.
	Outside []string
}

// Diff is the change a tool call would make to one file.
type Diff struct {
	Path string
	Old  string // empty for a new file
	New  string
}

// Question asks the user to pick among options, or to type an answer.
// The turn waits for the answer. ID is the ask tool call's ID. The
// question stays open after an interrupt, until it is answered or the
// next turn starts.
type Question struct {
	ID      string
	Text    string
	Options []Option
	Multi   bool // more than one option may be picked
}

// Option is one answer to a Question.
type Option struct {
	Label       string
	Description string
	Preview     string // longer text shown while the option is highlighted
}

// QuestionAnswered closes the question ID: the turn that waited for it
// goes on.
type QuestionAnswered struct{ ID string }

// TasksChanged gives the agent's whole task list after a change.
type TasksChanged struct{ Tasks []Task }

// Task is one item of an agent's task list.
type Task struct {
	Text  string
	State string // for example "pending", "doing", "done"
}

// Usage counts the tokens of one reply, with its estimated cost.
type Usage struct {
	Requested  string // "provider/model" the turn asked for
	Reported   string // the model ID the vendor says it used
	Input      int
	Output     int
	CacheRead  int
	CacheWrite int
	Cost       float64 // in US dollars; valid when Priced
	Priced     bool
}

// State is what an agent is doing.
type State string

// The states of an agent.
const (
	StateIdle      State = "idle"
	StateStreaming State = "streaming" // waiting for the model
	StateTool      State = "tool"      // running a tool
	StateApproval  State = "approval"  // waiting for the user's answer
)

// StateChanged says what the agent does now.
type StateChanged struct{ State State }

// QueueChanged gives the messages that wait for the running turn to end.
type QueueChanged struct{ Messages []string }

// ModelChanged gives the model the next turn uses. Failure is set when a
// switch could not be saved; the model stays as it was.
type ModelChanged struct {
	Model   string // "provider/model"
	Effort  string
	Failure *Failure
}

// ModeChanged gives the approval mode now in force. Failure is set when
// a change could not be saved; the mode stays as it was.
type ModeChanged struct {
	Mode    Mode
	Failure *Failure
}

// TitleChanged gives the conversation's title now in force. Failure is set
// when a rename could not be saved; the title stays as it was.
type TitleChanged struct {
	Title   string
	Failure *Failure
}

func (*TurnStarted) body()      {}
func (*TurnEnded) body()        {}
func (*TextDelta) body()        {}
func (*ThinkingDelta) body()    {}
func (*ToolCall) body()         {}
func (*ToolResult) body()       {}
func (*ApprovalRequest) body()  {}
func (*Question) body()         {}
func (*QuestionAnswered) body() {}
func (*TasksChanged) body()     {}
func (*Usage) body()            {}
func (*StateChanged) body()     {}
func (*QueueChanged) body()     {}
func (*ModelChanged) body()     {}
func (*ModeChanged) body()      {}
func (*TitleChanged) body()     {}
