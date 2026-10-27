package claudecode

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/emmyfong/snow/internal/transport"
	"github.com/emmyfong/snow/pkg/api"
)

// Hook events Snow listens to, as Claude Code names them in
// hook_event_name. HookEvents lists them in the order settings show them.
const (
	EventSessionStart     = "SessionStart"
	EventUserPromptSubmit = "UserPromptSubmit"
	EventPreToolUse       = "PreToolUse"
	EventPostToolUse      = "PostToolUse"
	EventNotification     = "Notification"
	EventStop             = "Stop"
	EventSessionEnd       = "SessionEnd"
)

// HookEvents are the events snow claude setup hooks.
var HookEvents = []string{
	EventSessionStart, EventUserPromptSubmit, EventPreToolUse, EventPostToolUse,
	EventNotification, EventStop, EventSessionEnd,
}

// Errors from Hook.Run. Callers match them with errors.Is.
var (
	// ErrNotInPane means snow hook runs outside a Snow pane: SNOW_PANE is
	// missing or not a pane ID. Nothing is sent.
	ErrNotInPane = errors.New("claudecode: not in a Snow pane")
	// ErrNoAnswer means the server did not take the report in time.
	ErrNoAnswer = errors.New("claudecode: the Snow server did not answer in time")
	// ErrNoInput means the hook's input did not arrive in time.
	ErrNoInput = errors.New("claudecode: no hook input in time")
	// ErrPanic means snow hook failed in a way Go reports as a panic. It
	// is an error, so snow hook can still exit 0.
	ErrPanic = errors.New("claudecode: internal failure")
)

// event is the part of a hook's input Snow reads. Claude Code sends more,
// such as the tool's input, which Snow skips.
type event struct {
	Name             string
	NotificationType string
}

// maxInput bounds how much of a hook's input StateFor reads. The event's
// name comes before the tool's input, which can be a whole file.
const maxInput = 64 << 10

// StateFor reads a hook's JSON input from r and returns the agent state it
// means. change is false when the event changes nothing, such as the
// reminder Claude Code sends a minute after a turn ends, or input that is
// not a hook event. An empty state with change set clears the pane's
// state. It reads at most maxInput bytes and stops after the first value;
// the caller drains the rest.
func StateFor(r io.Reader) (state string, change bool) {
	e := readEvent(io.LimitReader(r, maxInput))
	switch e.Name {
	case EventUserPromptSubmit, EventPreToolUse, EventPostToolUse:
		return api.AgentWorking, true
	case EventNotification:
		switch e.NotificationType {
		case "", "permission_prompt", "elicitation_dialog":
			return api.AgentNeedsYou, true
		}
		return "", false
	case EventStop, EventSessionStart:
		return api.AgentIdle, true
	case EventSessionEnd:
		return "", true
	}
	return "", false
}

// readEvent reads the top-level keys of one JSON object and keeps the two
// Snow needs. Input cut short, by the cap or a broken payload, keeps what
// was read before the cut.
func readEvent(r io.Reader) event {
	var e event
	dec := json.NewDecoder(r)
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return event{}
	}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return e
		}
		var skip json.RawMessage
		var dst any = &skip
		switch tok {
		case "hook_event_name":
			dst = &e.Name
		case "notification_type":
			dst = &e.NotificationType
		}
		if err := dec.Decode(dst); err != nil {
			return e
		}
	}
	return e
}

// sendBudget bounds a whole run of snow hook, from its start: reading the
// input, connecting, and sending. Claude Code waits for some hooks, such
// as PreToolUse, before it goes on.
const sendBudget = 500 * time.Millisecond

// Hook is one run of snow hook.
type Hook struct {
	Pane    string           // SNOW_PANE: the pane's ID
	Socket  string           // SNOW_SOCKET: the server's socket path
	Version string           // snow's version, for the handshake
	Now     func() time.Time // the time the report carries; nil means time.Now
}

// input is what the reading goroutine found.
type input struct {
	state  string
	change bool
	err    error
}

// Run reads the hook's input from stdin and sends the agent state it means
// to the Snow server, for the pane Claude Code runs in, all within
// sendBudget. It returns ErrNotInPane outside a Snow pane, and nil
// without connecting when the event changes nothing. Before it returns it
// lets Claude Code finish writing the input, within the same budget, so
// Claude Code never writes to a closed pipe.
func (h Hook) Run(stdin io.Reader) error {
	at := time.Now()
	if h.Now != nil {
		at = h.Now()
	}
	deadline := time.Now().Add(sendBudget)
	read := make(chan input, 1)
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		defer func() {
			if p := recover(); p != nil {
				select {
				case read <- input{err: fmt.Errorf("%w: %v", ErrPanic, p)}:
				default: // the input was already read; only the drain failed
				}
			}
		}()
		state, change := StateFor(stdin)
		read <- input{state: state, change: change}
		_, _ = io.Copy(io.Discard, stdin) // only drains; the event is read
	}()
	defer func() {
		select {
		case <-drained:
		case <-time.After(time.Until(deadline)):
		}
	}()
	var in input
	select {
	case in = <-read:
	case <-time.After(time.Until(deadline)):
		return ErrNoInput
	}
	if in.err != nil {
		return in.err
	}
	pane, err := strconv.Atoi(h.Pane)
	if err != nil || pane < 1 || h.Socket == "" {
		return ErrNotInPane
	}
	if !in.change {
		return nil
	}
	return send(h.Socket, h.Version, &api.AgentState{Pane: pane, State: in.state, At: at}, deadline)
}

// send connects to the server at socket and sends m, giving up at deadline.
func send(socket, version string, m *api.AgentState, deadline time.Time) error {
	left := time.Until(deadline)
	if left <= 0 {
		return ErrNoAnswer
	}
	raw, err := transport.DialTimeout(socket, left)
	if err != nil {
		return fmt.Errorf("claudecode: %w", err)
	}
	defer func() { _ = raw.Close() }()
	// Closing the connection ends a handshake that waits on a silent
	// server; the handshake's own deadline is longer than a hook may take.
	late := time.AfterFunc(time.Until(deadline), func() { _ = raw.Close() })
	defer late.Stop()
	conn := api.NewConn(raw)
	// The size is never used: this client does not attach. A valid one
	// keeps the server from logging each hook.
	hello := api.Hello{ClientVersion: version, Protocol: api.ProtocolVersion, Cols: 80, Rows: 24}
	if _, err := api.ClientHandshake(conn, hello); err != nil {
		if !late.Stop() {
			return fmt.Errorf("%w: %w", ErrNoAnswer, err)
		}
		return fmt.Errorf("claudecode: %w", err)
	}
	if err := conn.Send(m); err != nil {
		if !late.Stop() {
			return fmt.Errorf("%w: %w", ErrNoAnswer, err)
		}
		return fmt.Errorf("claudecode: send agent state: %w", err)
	}
	// Wait for the server to close the connection after it read the
	// report. Closing first can lose it: the server's writer fails on a
	// closed peer and closes the connection before its reader got there.
	if cw, ok := raw.(interface{ CloseWrite() error }); ok && cw.CloseWrite() == nil {
		_, _ = io.Copy(io.Discard, raw) // ends at the server's close, or at the deadline
	}
	return nil
}
