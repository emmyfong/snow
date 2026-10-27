package api

import (
	"encoding/json"
	"time"
)

// Agent kinds, in AgentInfo.Kind.
const (
	AgentKindClaudeCode = "claude-code" // a terminal pane that runs Claude Code
	AgentKindChat       = "chat"        // a Snow chat pane
)

// What an agent that needs the user waits for, in AgentInfo.Waits. Empty
// means Snow does not know, as for a Claude Code pane.
const (
	AgentWaitsApproval = "approval" // a tool call waits for a yes or no
	AgentWaitsQuestion = "question" // the agent asked the user a question
)

// AgentList lists every agent in the server's running sessions: Claude
// Code panes and Snow chat panes. The server sends it to every client,
// attached or not, when the client connects and whenever an entry
// changes. Agents are ordered by session name, window index, and pane ID.
type AgentList struct {
	Agents []AgentInfo `json:"agents"`
}

// Type implements Message.
func (*AgentList) Type() MsgType { return TypeAgentList }
func (*AgentList) message()      {}

// MarshalJSON writes Agents as [] when it is empty.
func (l AgentList) MarshalJSON() ([]byte, error) {
	type wire AgentList
	if l.Agents == nil {
		l.Agents = []AgentInfo{}
	}
	return json.Marshal(wire(l))
}

// AgentInfo is one agent in an AgentList.
//
// State is one of the Agent* states, or empty while a Claude Code pane has
// not reported yet. Since is when the agent entered State, by the server's
// clock; it restarts when the server restarts. Waits says what a
// needs-you agent waits for, when Snow knows. Worktree is the git worktree
// the agent works in, when Snow made one for it. Model and Cost are set
// for chat panes only: the model the next turn uses, and the estimated
// cost of the conversation's priced replies, in US dollars.
type AgentInfo struct {
	Session    string    `json:"session"`
	Window     int       `json:"window"`
	WindowName string    `json:"windowName"`
	Pane       int       `json:"pane"`
	Kind       string    `json:"kind"`
	Profile    string    `json:"profile,omitempty"`
	Folder     string    `json:"folder,omitempty"`
	Worktree   string    `json:"worktree,omitempty"`
	State      string    `json:"state,omitempty"`
	Since      time.Time `json:"since"`
	Waits      string    `json:"waits,omitempty"`
	Model      string    `json:"model,omitempty"`
	Cost       float64   `json:"cost,omitempty"`
}
