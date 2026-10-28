package api

import (
	"encoding/json"
	"errors"
	"fmt"
)

// The pane tools let an agent that runs in a pane, such as Claude Code
// through snow mcp, use the other terminal panes of its session. Every
// request names the agent's own pane as Caller, from SNOW_PANE; the server
// answers only about panes in the caller's session. A client sends one
// request per connection and does not attach. It reads past the session
// lists every connection gets until the reply, or an Error, arrives.

// Pane tool error codes, in Error.Code. CodeNotTerminal also applies: a
// chat pane has no screen.
const (
	// CodeNoPane means the caller or the named pane is not in the server.
	CodeNoPane = "no-pane"
	// CodeOtherSession means the named pane is in another session than the
	// caller. Tools reach only the caller's session.
	CodeOtherSession = "other-session"
)

// ErrInvalidPaneTool is returned by the Validate methods of the pane tool
// requests.
var ErrInvalidPaneTool = errors.New("api: invalid pane tool request")

// PanesRequest asks for the terminal panes of the caller's session. The
// server answers with Panes, or an Error with CodeNoPane when the caller
// is not one of its panes.
type PanesRequest struct {
	Caller int `json:"caller"`
}

// Panes answers a PanesRequest: every terminal pane of the session, the
// caller's own included, in window order and, in a window, in layout
// order.
type Panes struct {
	Session string        `json:"session"`
	Caller  int           `json:"caller"`
	Panes   []PaneSummary `json:"panes"`
}

// PaneSummary describes one terminal pane for an agent. Window and
// WindowName say which window holds it; Shown is set for the window the
// session shows. Rect is its place in that window. Agent is set when an
// agent, such as Claude Code, runs in it; AgentState then says what the
// agent does. Ended is set for a pane whose program ended and that waits
// to be restarted.
type PaneSummary struct {
	ID         int    `json:"id"`
	Window     int    `json:"window"`
	WindowName string `json:"windowName"`
	Shown      bool   `json:"shown,omitempty"`
	Profile    string `json:"profile"`
	Folder     string `json:"folder,omitempty"`
	Rect       Rect   `json:"rect"`
	Focused    bool   `json:"focused,omitempty"`
	Agent      bool   `json:"agent,omitempty"`
	AgentState string `json:"agentState,omitempty"`
	Ended      bool   `json:"ended,omitempty"`
}

// Type implements Message.
func (*PanesRequest) Type() MsgType { return TypePanesRequest }
func (*PanesRequest) message()      {}

// Type implements Message.
func (*Panes) Type() MsgType { return TypePanes }
func (*Panes) message()      {}

// MarshalJSON writes Panes as [] when it is empty.
func (p Panes) MarshalJSON() ([]byte, error) {
	type wire Panes
	if p.Panes == nil {
		p.Panes = []PaneSummary{}
	}
	return json.Marshal(wire(p))
}

// Validate checks the caller.
func (r *PanesRequest) Validate() error { return validCaller(r.Caller) }

func validCaller(caller int) error {
	if caller < 1 {
		return fmt.Errorf("%w: no caller pane", ErrInvalidPaneTool)
	}
	return nil
}
