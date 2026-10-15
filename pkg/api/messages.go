package api

import (
	"errors"
	"fmt"
	"time"
)

// MsgType identifies a message on the wire.
type MsgType byte

// Message types. Never renumber a released type.
const (
	TypeHello MsgType = iota + 1
	TypeWelcome
	TypeAttach
	TypeInput
	TypeMouse
	TypeResize
	TypeCommand
	TypeLayout
	TypePaneUpdate
	TypeSessionList
	TypeBell
	TypeError
	TypeProfileList
)

// Message is any value that can travel in a frame. Only this package's types
// implement it: the unexported method seals the interface, so every Message
// has a known wire type.
type Message interface {
	Type() MsgType
	message()
}

// newMessage returns an empty message of type t, or nil if t is unknown.
func newMessage(t MsgType) Message {
	switch t {
	case TypeHello:
		return &Hello{}
	case TypeWelcome:
		return &Welcome{}
	case TypeAttach:
		return &Attach{}
	case TypeInput:
		return &Input{}
	case TypeMouse:
		return &Mouse{}
	case TypeResize:
		return &Resize{}
	case TypeCommand:
		return &Command{}
	case TypeLayout:
		return &Layout{}
	case TypePaneUpdate:
		return &PaneUpdate{}
	case TypeSessionList:
		return &SessionList{}
	case TypeBell:
		return &Bell{}
	case TypeError:
		return &Error{}
	case TypeProfileList:
		return &ProfileList{}
	}
	return nil
}

// Hello is the client's first message.
type Hello struct {
	ClientVersion string `json:"clientVersion"`
	Protocol      int    `json:"protocol"`
	Cols          int    `json:"cols"`
	Rows          int    `json:"rows"`
}

// Welcome is the server's answer to Hello.
type Welcome struct {
	ServerVersion string `json:"serverVersion"`
	Protocol      int    `json:"protocol"`
}

// Attach asks to show a session, creating it when Create is set. An empty
// Session with Create set asks for a new session with the lowest free number.
// Dir is the folder a new session starts in, usually the client's working
// folder; empty, or a folder the server cannot use, means the user's home.
type Attach struct {
	Session string `json:"session"`
	Create  bool   `json:"create,omitempty"`
	Dir     string `json:"dir,omitempty"`
}

// Input is one key press or one paste for a pane. Exactly one of Key and
// Paste is set; Validate checks it. The server's emulator encodes keys for
// the pane's mode. Paste is UTF-8 text: invalid bytes become U+FFFD in JSON.
type Input struct {
	Pane  int    `json:"pane"`
	Key   *Key   `json:"key,omitempty"`
	Paste string `json:"paste,omitempty"`
}

// Mouse is one mouse event at a cell inside a pane. X and Y count from 0,
// from the pane's top-left cell. Clients send it only for panes whose
// PaneUpdate says Mouse; the server drops it for any other pane.
type Mouse struct {
	Pane   int    `json:"pane"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Button string `json:"button"` // one of the Button* values
	Action string `json:"action"` // one of the Action* values
}

// Mouse buttons.
const (
	ButtonNone      = "none"
	ButtonLeft      = "left"
	ButtonMiddle    = "middle"
	ButtonRight     = "right"
	ButtonWheelUp   = "wheelup"
	ButtonWheelDown = "wheeldown"
)

// Mouse actions.
const (
	ActionPress   = "press"
	ActionRelease = "release"
	ActionMotion  = "motion"
)

// Resize reports the client's terminal size in cells.
type Resize struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

// Command asks the server to change the session, for example to split a pane.
// Target is a pane ID or a window index, depending on Op. The
// Op* constants list the operations and their Args; Validate checks one.
type Command struct {
	Op     string            `json:"op"`
	Target int               `json:"target,omitempty"`
	Args   map[string]string `json:"args,omitempty"`
}

// Layout describes the attached session's windows and where each pane is.
// Active is the index of the shown window.
type Layout struct {
	Session string       `json:"session"`
	Active  int          `json:"active"`
	Windows []WindowInfo `json:"windows"`
}

// WindowInfo is one window in a Layout.
type WindowInfo struct {
	Index  int        `json:"index"`
	Name   string     `json:"name"`
	Zoomed bool       `json:"zoomed,omitempty"`
	Panes  []PaneInfo `json:"panes"`
}

// PaneInfo is one pane's place on screen.
type PaneInfo struct {
	ID      int    `json:"id"`
	Rect    Rect   `json:"rect"`
	Profile string `json:"profile"`
	Folder  string `json:"folder,omitempty"` // the program's folder, when the shell reports it
	Focused bool   `json:"focused,omitempty"`
}

// Rect is a cell rectangle; X and Y are the top-left corner.
type Rect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// PaneUpdate carries a pane's changed lines.
type PaneUpdate struct {
	Pane   int          `json:"pane"`
	Lines  []LineUpdate `json:"lines"`
	Cursor *Cursor      `json:"cursor,omitempty"`
	// Mouse tells whether the program in the pane asked for mouse events.
	// Every update carries the current answer, and a missing field means no.
	// A client that sees true sends the pane's clicks and wheel as Mouse
	// messages instead of acting on them.
	Mouse bool `json:"mouse,omitempty"`
}

// LineUpdate is one screen row, styled with ANSI escape sequences. Text is
// UTF-8.
type LineUpdate struct {
	Row  int    `json:"row"`
	Text string `json:"text"`
}

// Cursor is the cursor position inside a pane.
type Cursor struct {
	X       int  `json:"x"`
	Y       int  `json:"y"`
	Visible bool `json:"visible"`
}

// Session states in SessionInfo.
const (
	StateRunning = "running"
	StateSaved   = "saved"
)

// SessionList lists the server's sessions.
type SessionList struct {
	Sessions []SessionInfo `json:"sessions"`
}

// SessionInfo summarizes one session.
type SessionInfo struct {
	Name     string    `json:"name"`
	Windows  int       `json:"windows"`
	Attached int       `json:"attached"`
	State    string    `json:"state"`
	LastUsed time.Time `json:"lastUsed"`
}

// Bell reports that a pane rang the terminal bell.
type Bell struct {
	Pane int `json:"pane"`
}

// Error reports a failed request. Code is stable for programs; Message is
// for people.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error codes.
const (
	CodeBadHandshake = "bad-handshake" // the first message was not a readable Hello
	CodeNoSession    = "no-session"    // Attach named a session that does not exist
	CodeAttachFailed = "attach-failed" // the server could not create the session
	CodeBadSize      = "bad-size"      // a Hello or Resize size is outside 1..MaxCols by 1..MaxRows
	CodeBadCommand   = "bad-command"   // a Command failed Validate
	// CodeCommandFailed means a valid Command could not be applied, for
	// example a split that would leave a pane too small. Nothing changed.
	CodeCommandFailed = "command-failed"
	// CodeSessionEnded is the last message before the server closes the
	// connection because the attached session ended. A close without it
	// means the connection was lost.
	CodeSessionEnded = "session-ended"
)

// Error makes a server's Error message usable as a Go error.
func (e *Error) Error() string { return fmt.Sprintf("server: %s: %s", e.Code, e.Message) }

// Type implements Message.
func (*Hello) Type() MsgType { return TypeHello }
func (*Hello) message()      {}

// Type implements Message.
func (*Welcome) Type() MsgType { return TypeWelcome }
func (*Welcome) message()      {}

// Type implements Message.
func (*Attach) Type() MsgType { return TypeAttach }
func (*Attach) message()      {}

// Type implements Message.
func (*Input) Type() MsgType { return TypeInput }
func (*Input) message()      {}

// Type implements Message.
func (*Mouse) Type() MsgType { return TypeMouse }
func (*Mouse) message()      {}

// Type implements Message.
func (*Resize) Type() MsgType { return TypeResize }
func (*Resize) message()      {}

// Type implements Message.
func (*Command) Type() MsgType { return TypeCommand }
func (*Command) message()      {}

// Type implements Message.
func (*Layout) Type() MsgType { return TypeLayout }
func (*Layout) message()      {}

// Type implements Message.
func (*PaneUpdate) Type() MsgType { return TypePaneUpdate }
func (*PaneUpdate) message()      {}

// Type implements Message.
func (*SessionList) Type() MsgType { return TypeSessionList }
func (*SessionList) message()      {}

// Type implements Message.
func (*Bell) Type() MsgType { return TypeBell }
func (*Bell) message()      {}

// Type implements Message.
func (*Error) Type() MsgType { return TypeError }
func (*Error) message()      {}

// ErrInvalidInput is returned by Input.Validate.
var ErrInvalidInput = errors.New("api: invalid input")

// Validate checks that i names a pane and carries exactly one valid key or
// one paste. Servers call it before acting on Input.
func (i *Input) Validate() error {
	switch {
	case i.Pane <= 0:
		return fmt.Errorf("%w: no pane", ErrInvalidInput)
	case (i.Key == nil) == (i.Paste == ""):
		return fmt.Errorf("%w: need exactly one of key and paste", ErrInvalidInput)
	case i.Key != nil && !i.Key.Valid():
		// The key's text is what the user typed; servers log this error.
		return fmt.Errorf("%w: unknown key", ErrInvalidInput)
	}
	return nil
}

// ErrInvalidMouse is returned by Mouse.Validate.
var ErrInvalidMouse = errors.New("api: invalid mouse event")

// Validate checks that m names a pane, a cell, and a known button and action.
// Servers call it before acting on a Mouse.
func (m *Mouse) Validate() error {
	switch {
	case m.Pane <= 0:
		return fmt.Errorf("%w: no pane", ErrInvalidMouse)
	case m.X < 0 || m.Y < 0:
		return fmt.Errorf("%w: cell %d,%d is outside the pane", ErrInvalidMouse, m.X, m.Y)
	}
	switch m.Button {
	case ButtonNone, ButtonLeft, ButtonMiddle, ButtonRight, ButtonWheelUp, ButtonWheelDown:
	default:
		return fmt.Errorf("%w: unknown button %q", ErrInvalidMouse, m.Button)
	}
	switch m.Action {
	case ActionPress, ActionRelease, ActionMotion:
	default:
		return fmt.Errorf("%w: unknown action %q", ErrInvalidMouse, m.Action)
	}
	return nil
}
