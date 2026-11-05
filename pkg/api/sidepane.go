package api

import (
	"encoding/json"
	"errors"
	"fmt"
)

// A pane kind other than PaneTerminal and PaneChat is a side pane, such as
// a file tree. Every side pane uses the same two messages: SidePaneState
// carries its state to clients and SidePaneAction carries a user's action
// to the server. Each kind defines its state as a named struct in this
// package, so a new kind needs no new message type and no new
// ProtocolVersion.

// MaxKindName bounds a pane kind's name.
const MaxKindName = 32

// ErrInvalidKind is returned for a pane kind name that is malformed.
var ErrInvalidKind = errors.New("invalid pane kind")

// ErrInvalidSidePane is returned by the side pane messages' Validate.
var ErrInvalidSidePane = errors.New("invalid side pane message")

// CheckKind reports whether name is a well-formed pane kind: 1 to
// MaxKindName lowercase letters, digits, or hyphens, starting with a
// letter. A well-formed kind can still be one the server does not have.
func CheckKind(name string) error {
	if name == "" || len(name) > MaxKindName {
		return fmt.Errorf("%w: %q must be 1 to %d characters", ErrInvalidKind, name, MaxKindName)
	}
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case i > 0 && (r >= '0' && r <= '9' || r == '-'):
		default:
			return fmt.Errorf("%w: %q must be lowercase letters, digits, and hyphens, starting with a letter", ErrInvalidKind, name)
		}
	}
	return nil
}

// PaneKindInfo is one kind of pane a new pane can be.
type PaneKindInfo struct {
	Name  string `json:"name"`  // the value of ArgKind
	Title string `json:"title"` // what a picker shows
}

// PaneKinds lists the kinds of pane this server opens, in a stable order
// for a picker. The server sends it after each Attach.
type PaneKinds struct {
	Kinds []PaneKindInfo `json:"kinds"`
}

// Type implements Message.
func (*PaneKinds) Type() MsgType { return TypePaneKinds }
func (*PaneKinds) message()      {}

// MarshalJSON writes Kinds as [] when it is empty.
func (l PaneKinds) MarshalJSON() ([]byte, error) {
	type wire PaneKinds
	if l.Kinds == nil {
		l.Kinds = []PaneKindInfo{}
	}
	return json.Marshal(wire(l))
}

// SidePaneState is the whole state of side pane Pane, of kind Kind. The
// server sends it after the Layout that lists the pane, and again each time
// the state changes. Rev grows with each change, so a client can drop an
// older one. State is the kind's state struct, as JSON; decode it with
// DecodeSideState.
type SidePaneState struct {
	Pane  int             `json:"pane"`
	Kind  string          `json:"kind"`
	Rev   uint64          `json:"rev"`
	State json.RawMessage `json:"state"`
}

// Type implements Message.
func (*SidePaneState) Type() MsgType { return TypeSidePaneState }
func (*SidePaneState) message()      {}

// Validate checks the envelope. The kind's own Validate checks State.
func (s *SidePaneState) Validate() error {
	if s.Pane <= 0 {
		return fmt.Errorf("%w: no pane", ErrInvalidSidePane)
	}
	if err := CheckKind(s.Kind); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidSidePane, err)
	}
	if !json.Valid(s.State) {
		return fmt.Errorf("%w: state is not JSON", ErrInvalidSidePane)
	}
	return nil
}

// SideState is the state struct of one side pane kind.
type SideState interface {
	// Kind is the pane kind the state belongs to.
	Kind() string
	Validate() error
}

// NewSidePaneState wraps state as the SidePaneState of pane at rev.
func NewSidePaneState(pane int, rev uint64, state SideState) (*SidePaneState, error) {
	if err := state.Validate(); err != nil {
		return nil, err
	}
	b, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidSidePane, err)
	}
	return &SidePaneState{Pane: pane, Kind: state.Kind(), Rev: rev, State: b}, nil
}

// DecodeSideState decodes s's state into into, which must be the state
// struct of s's kind, and validates it.
func DecodeSideState(s *SidePaneState, into SideState) error {
	if s.Kind != into.Kind() {
		return fmt.Errorf("%w: state of kind %q, not %q", ErrInvalidSidePane, s.Kind, into.Kind())
	}
	if err := json.Unmarshal(s.State, into); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidSidePane, err)
	}
	return into.Validate()
}

// MaxSideArgs bounds the arguments of a SidePaneAction.
const MaxSideArgs = 16

// SidePaneAction asks side pane Pane to do Action, a name its kind defines,
// with Args. The server answers with a new SidePaneState, or an Error.
type SidePaneAction struct {
	Pane   int               `json:"pane"`
	Action string            `json:"action"`
	Args   map[string]string `json:"args,omitempty"`
}

// Type implements Message.
func (*SidePaneAction) Type() MsgType { return TypeSidePaneAction }
func (*SidePaneAction) message()      {}

// Validate checks the envelope. The pane's kind checks Action and Args.
func (a *SidePaneAction) Validate() error {
	switch {
	case a.Pane <= 0:
		return fmt.Errorf("%w: no pane", ErrInvalidSidePane)
	case a.Action == "":
		return fmt.Errorf("%w: no action", ErrInvalidSidePane)
	case len(a.Args) > MaxSideArgs:
		return fmt.Errorf("%w: more than %d arguments", ErrInvalidSidePane, MaxSideArgs)
	}
	return nil
}
