package api

// ClientFocus reports whether the terminal of a client has focus, from
// Bubble Tea's focus and blur events. The server skips a desktop
// notification while a client whose terminal has focus shows the agent's
// pane, because the user already sees it.
type ClientFocus struct {
	Focused bool `json:"focused"`
}

// Type implements Message.
func (*ClientFocus) Type() MsgType { return TypeClientFocus }
func (*ClientFocus) message()      {}
