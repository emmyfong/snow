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
	// CodeOwnPane means send_keys named the pane the agent runs in.
	CodeOwnPane = "own-pane"
	// CodeAgentPane means send_keys named a pane another agent runs in.
	CodeAgentPane = "agent-pane"
	// CodePaneEnded means send_keys named a pane whose program ended and
	// that waits for the user to restore it.
	CodePaneEnded = "pane-ended"
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

// ReadPaneRequest asks for a plain-text copy of one terminal pane of the
// caller's session. Scrollback is how many lines above the screen to
// return; 0 asks for the server's default, and the server caps it.
type ReadPaneRequest struct {
	Caller     int `json:"caller"`
	Pane       int `json:"pane"`
	Scrollback int `json:"scrollback,omitempty"`
}

// ReadPane answers a ReadPaneRequest with a copy of one terminal pane at
// one moment. Lines carry no styles. Screen holds the rows now on the
// screen, top row first. Scrollback holds the newest lines above the
// screen, oldest first. Omitted counts the lines not returned because the
// request or the reply size left them out; they are the oldest lines.
// Cursor is zero-based, and CursorVisible is set when the program shows it.
// Ended is set for a pane whose program ended and that waits to restart.
type ReadPane struct {
	Pane          int      `json:"pane"`
	Session       string   `json:"session"`
	Profile       string   `json:"profile"`
	Folder        string   `json:"folder,omitempty"`
	Cols          int      `json:"cols"`
	Rows          int      `json:"rows"`
	CursorX       int      `json:"cursorX"`
	CursorY       int      `json:"cursorY"`
	CursorVisible bool     `json:"cursorVisible,omitempty"`
	Screen        []string `json:"screen"`
	Scrollback    []string `json:"scrollback,omitempty"`
	Omitted       int      `json:"omitted,omitempty"`
	Ended         bool     `json:"ended,omitempty"`
}

// Type implements Message.
func (*ReadPaneRequest) Type() MsgType { return TypeReadPaneRequest }
func (*ReadPaneRequest) message()      {}

// Type implements Message.
func (*ReadPane) Type() MsgType { return TypeReadPane }
func (*ReadPane) message()      {}

// MarshalJSON writes an empty Screen as [].
func (r ReadPane) MarshalJSON() ([]byte, error) {
	type wire ReadPane
	if r.Screen == nil {
		r.Screen = []string{}
	}
	return json.Marshal(wire(r))
}

// Validate checks the caller, the pane, and the scrollback count.
func (r *ReadPaneRequest) Validate() error {
	if err := validCaller(r.Caller); err != nil {
		return err
	}
	if r.Pane < 1 {
		return fmt.Errorf("%w: no pane", ErrInvalidPaneTool)
	}
	if r.Scrollback < 0 {
		return fmt.Errorf("%w: negative scrollback %d", ErrInvalidPaneTool, r.Scrollback)
	}
	return nil
}

func validCaller(caller int) error {
	if caller < 1 {
		return fmt.Errorf("%w: no caller pane", ErrInvalidPaneTool)
	}
	return nil
}

// SendKeysRequest types into one terminal pane of the caller's session: it
// pastes Text, then presses Keys in order. Keys are named as ParseKey takes
// them. The server refuses the caller's own pane, another agent's pane, a
// chat pane, and an ended pane.
type SendKeysRequest struct {
	Caller int    `json:"caller"`
	Pane   int    `json:"pane"`
	Text   string `json:"text,omitempty"`
	Keys   []Key  `json:"keys,omitempty"`
}

// SendKeys answers a SendKeysRequest. Dropped counts the bytes the program
// did not read because it stopped reading; they never reached it.
type SendKeys struct {
	Pane    int `json:"pane"`
	Dropped int `json:"dropped,omitempty"`
}

// Type implements Message.
func (*SendKeysRequest) Type() MsgType { return TypeSendKeysRequest }
func (*SendKeysRequest) message()      {}

// Type implements Message.
func (*SendKeys) Type() MsgType { return TypeSendKeys }
func (*SendKeys) message()      {}

// Validate checks the caller, the pane, the keys, and that there is input.
func (r *SendKeysRequest) Validate() error {
	if err := validCaller(r.Caller); err != nil {
		return err
	}
	if r.Pane < 1 {
		return fmt.Errorf("%w: no pane", ErrInvalidPaneTool)
	}
	if r.Text == "" && len(r.Keys) == 0 {
		return fmt.Errorf("%w: no text and no keys", ErrInvalidPaneTool)
	}
	for _, k := range r.Keys {
		if !k.Valid() {
			return fmt.Errorf("%w: bad key %+v", ErrInvalidPaneTool, k)
		}
	}
	return nil
}

// Wait results, in WaitFor.Result.
const (
	// WaitFound means the pane printed the text.
	WaitFound = "found"
	// WaitQuiet means the pane did not change for the quiet period.
	WaitQuiet = "quiet"
	// WaitExited means the pane's program exited before the condition held.
	WaitExited = "exited"
	// WaitTimeout means the time ran out while the program kept running.
	WaitTimeout = "timeout"
)

// Wait limits in milliseconds: a request without TimeoutMS gets
// WaitDefaultMS, and no request may ask for more than WaitMaxMS.
const (
	WaitDefaultMS = 30_000
	WaitMaxMS     = 600_000
)

// WaitForRequest waits until one terminal pane of the caller's session
// prints Text or stops changing for QuietMS milliseconds. Give exactly one
// of Text and QuietMS. A zero TimeoutMS takes the server's default. Text
// already on the pane counts only with IncludeExisting.
type WaitForRequest struct {
	Caller          int    `json:"caller"`
	Pane            int    `json:"pane"`
	Text            string `json:"text,omitempty"`
	QuietMS         int    `json:"quietMs,omitempty"`
	TimeoutMS       int    `json:"timeoutMs,omitempty"`
	IncludeExisting bool   `json:"includeExisting,omitempty"`
}

// WaitFor answers a WaitForRequest. Line holds the matching line for
// WaitFound. Exit holds the program's status for WaitExited when Snow knows
// it.
type WaitFor struct {
	Pane   int    `json:"pane"`
	Result string `json:"result"`
	Line   string `json:"line,omitempty"`
	Exit   *int   `json:"exit,omitempty"`
}

// Type implements Message.
func (*WaitForRequest) Type() MsgType { return TypeWaitForRequest }
func (*WaitForRequest) message()      {}

// Type implements Message.
func (*WaitFor) Type() MsgType { return TypeWaitFor }
func (*WaitFor) message()      {}

// Validate checks the caller, the pane, and exactly one condition.
func (r *WaitForRequest) Validate() error {
	if err := validCaller(r.Caller); err != nil {
		return err
	}
	if r.Pane < 1 {
		return fmt.Errorf("%w: no pane", ErrInvalidPaneTool)
	}
	if (r.Text == "") == (r.QuietMS == 0) {
		return fmt.Errorf("%w: need exactly one of text and quietMs", ErrInvalidPaneTool)
	}
	if r.QuietMS < 0 {
		return fmt.Errorf("%w: quietMs cannot be negative", ErrInvalidPaneTool)
	}
	if r.TimeoutMS < 0 {
		return fmt.Errorf("%w: timeoutMs cannot be negative", ErrInvalidPaneTool)
	}
	return nil
}
