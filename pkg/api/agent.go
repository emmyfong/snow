package api

import (
	"errors"
	"fmt"
	"time"
)

// Agent states, in AgentState.State, PaneInfo.AgentState, and
// WindowInfo.AgentState.
// An empty state means no agent runs there, or none has reported yet.
const (
	AgentIdle     = "idle"      // the agent waits for a new task
	AgentWorking  = "working"   // the agent runs a turn
	AgentNeedsYou = "needs-you" // the agent waits for a permission answer or another answer from the user
)

// ErrInvalidAgentState is returned by AgentState.Validate.
var ErrInvalidAgentState = errors.New("api: invalid agent state")

// AgentState reports what the agent in a terminal pane does, for example
// from a Claude Code hook through snow hook. State is one of the Agent*
// values, or empty to clear the pane's state when the agent ends. At is
// when the hook ran: a report older than the pane's last one is ignored,
// because two hooks can race. The server ignores a pane it does not have,
// and answers nothing.
type AgentState struct {
	Pane  int       `json:"pane"`
	State string    `json:"state"`
	At    time.Time `json:"at"`
	// Commit is set when the hook was for a tool call that runs git
	// commit, or nil.
	Commit *AgentCommit `json:"commit,omitempty"`
}

// Phases of an AgentCommit: read before the tool call or after it.
const (
	CommitBefore = "before"
	CommitAfter  = "after"
)

// AgentCommit is what snow hook read of git around an agent's tool call
// that runs git commit, in the folder the agent works in. Before the call
// (Phase CommitBefore) it reads HEAD into Before; after it (CommitAfter),
// HEAD into After and HEAD's first parent into Parent. A HEAD that git
// cannot read is empty. The server counts a commit as the agent's only
// when After's parent is the Before of the same pane's last report: one
// new commit on top of what was there.
type AgentCommit struct {
	Phase  string `json:"phase"`
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
	Parent string `json:"parent,omitempty"`
}

// Validate checks the phase, that only its own hashes are set, and that
// each hash is a git object name.
func (c *AgentCommit) Validate() error {
	switch {
	case c.Phase == CommitBefore && c.After == "" && c.Parent == "":
	case c.Phase == CommitAfter && c.Before == "":
	default:
		return fmt.Errorf("%w: commit report of phase %q", ErrInvalidAgentState, c.Phase)
	}
	for _, h := range []string{c.Before, c.After, c.Parent} {
		if h != "" && !IsCommitHash(h) {
			return fmt.Errorf("%w: %q is not a commit hash", ErrInvalidAgentState, h)
		}
	}
	return nil
}

// IsCommitHash reports whether h is a full git object name: 40 hex
// digits for SHA-1, or 64 for SHA-256, in lower case.
func IsCommitHash(h string) bool {
	if len(h) != 40 && len(h) != 64 {
		return false
	}
	for _, r := range h {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// Type implements Message.
func (*AgentState) Type() MsgType { return TypeAgentState }
func (*AgentState) message()      {}

// Validate checks the pane ID and the state.
func (a *AgentState) Validate() error {
	if a.Pane < 1 {
		return fmt.Errorf("%w: no pane", ErrInvalidAgentState)
	}
	switch a.State {
	case "", AgentIdle, AgentWorking, AgentNeedsYou:
	default:
		return fmt.Errorf("%w: unknown state %q", ErrInvalidAgentState, a.State)
	}
	if a.Commit != nil {
		return a.Commit.Validate()
	}
	return nil
}

// urgency orders the agent states: the user must see the higher first.
var urgency = map[string]int{AgentIdle: 1, AgentWorking: 2, AgentNeedsYou: 3}

// MoreUrgent returns the state of a and b the user must see first:
// needs-you, then working, then idle, then none.
func MoreUrgent(a, b string) string {
	if urgency[b] > urgency[a] {
		return b
	}
	return a
}
