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
		return nil
	}
	return fmt.Errorf("%w: unknown state %q", ErrInvalidAgentState, a.State)
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
