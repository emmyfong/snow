package claudecode

import (
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/transport"
	"github.com/emmyfong/snow/pkg/api"
)

func TestStateFor(t *testing.T) {
	tests := []struct {
		name, payload, want string
		change              bool
	}{
		{"prompt", `{"hook_event_name":"UserPromptSubmit","prompt":"fix it"}`, api.AgentWorking, true},
		{"before a tool", `{"hook_event_name":"PreToolUse","tool_name":"Bash"}`, api.AgentWorking, true},
		{"after a tool: the user answered", `{"hook_event_name":"PostToolUse","tool_name":"Bash"}`, api.AgentWorking, true},
		{"permission prompt", `{"hook_event_name":"Notification","notification_type":"permission_prompt","message":"Claude needs your permission"}`, api.AgentNeedsYou, true},
		{"elicitation dialog", `{"hook_event_name":"Notification","notification_type":"elicitation_dialog"}`, api.AgentNeedsYou, true},
		{"notification with no type", `{"hook_event_name":"Notification","message":"Claude needs your permission"}`, api.AgentNeedsYou, true},
		{"idle prompt changes nothing", `{"hook_event_name":"Notification","notification_type":"idle_prompt"}`, "", false},
		{"other notification changes nothing", `{"hook_event_name":"Notification","notification_type":"auth_success"}`, "", false},
		{"turn ends", `{"hook_event_name":"Stop"}`, api.AgentIdle, true},
		{"session starts", `{"hook_event_name":"SessionStart","source":"startup"}`, api.AgentIdle, true},
		{"session ends clears", `{"hook_event_name":"SessionEnd","reason":"exit"}`, "", true},
		{"unknown event", `{"hook_event_name":"PreCompact"}`, "", false},
		{"malformed", `{"hook_event_name":`, "", false},
		{"not JSON", `hello`, "", false},
		{"empty", ``, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, change := StateFor(strings.NewReader(tt.payload))
			if got != tt.want || change != tt.change {
				t.Fatalf("StateFor() = %q, %v; want %q, %v", got, change, tt.want, tt.change)
			}
		})
	}
}

// fakeSnow listens at a short socket path and returns each AgentState a
// client sends after the handshake.
func fakeSnow(t *testing.T, protocol int) (string, <-chan *api.AgentState) {
	t.Helper()
	path := apitest.SocketPath(t)
	l, err := transport.Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	got := make(chan *api.AgentState, 8)
	go func() {
		for {
			raw, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = raw.Close() }()
				conn := api.NewConn(raw)
				if _, err := api.ServerHandshake(conn, api.Welcome{ServerVersion: "test", Protocol: protocol}); err != nil {
					return
				}
				for {
					m, err := conn.Receive()
					if err != nil {
						return
					}
					if a, ok := m.(*api.AgentState); ok {
						got <- a
					}
				}
			}()
		}
	}()
	return path, got
}

func TestHookSends(t *testing.T) {
	path, got := fakeSnow(t, api.ProtocolVersion)
	at := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	h := Hook{Pane: "7", Socket: path, Version: "test", Now: func() time.Time { return at }}
	if err := h.Run(strings.NewReader(`{"hook_event_name":"Notification","notification_type":"permission_prompt"}`)); err != nil {
		t.Fatal(err)
	}
	select {
	case m := <-got:
		if m.Pane != 7 || m.State != api.AgentNeedsYou || !m.At.Equal(at) {
			t.Fatalf("sent %+v, want pane 7 needs-you at %v", m, at)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("nothing sent")
	}
}

// TestHookOutsideSnow: with no SNOW_PANE, snow hook sends nothing.
func TestHookOutsideSnow(t *testing.T) {
	path, got := fakeSnow(t, api.ProtocolVersion)
	for _, pane := range []string{"", "x", "0", "-3"} {
		h := Hook{Pane: pane, Socket: path, Version: "test"}
		if err := h.Run(strings.NewReader(`{"hook_event_name":"Stop"}`)); !errors.Is(err, ErrNotInPane) {
			t.Errorf("pane %q: Run() = %v, want ErrNotInPane", pane, err)
		}
	}
	select {
	case m := <-got:
		t.Fatalf("sent %+v outside a pane", m)
	case <-time.After(200 * time.Millisecond):
	}
}

// TestHookNoChangeSendsNothing: an event that changes no state does not
// even connect.
func TestHookNoChangeSendsNothing(t *testing.T) {
	h := Hook{Pane: "7", Socket: "/no/such/snow.sock", Version: "test"}
	if err := h.Run(strings.NewReader(`{"hook_event_name":"Notification","notification_type":"idle_prompt"}`)); err != nil {
		t.Fatalf("Run() = %v, want nil: nothing to send", err)
	}
}

// TestHookOtherProtocol: a server of another protocol gets nothing.
func TestHookOtherProtocol(t *testing.T) {
	path, got := fakeSnow(t, api.ProtocolVersion+1)
	h := Hook{Pane: "7", Socket: path, Version: "test"}
	if err := h.Run(strings.NewReader(`{"hook_event_name":"Stop"}`)); !errors.Is(err, api.ErrVersionMismatch) {
		t.Fatalf("Run() = %v, want a version mismatch", err)
	}
	select {
	case m := <-got:
		t.Fatalf("sent %+v to a server of another protocol", m)
	case <-time.After(200 * time.Millisecond):
	}
}

// TestHookBudget: Claude Code waits for a PreToolUse hook before the tool
// runs, so snow hook gives up within one budget from its start, whatever
// it waits on: a server that accepts and stays silent, a socket nobody
// listens on, or input that never ends.
func TestHookBudget(t *testing.T) {
	if raceDetector {
		t.Skip("timing budget; -race slows the code")
	}
	silent := apitest.SocketPath(t)
	l, err := transport.Listen(silent)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		var held []net.Conn
		for {
			c, err := l.Accept()
			if err != nil {
				for _, c := range held {
					_ = c.Close()
				}
				return
			}
			held = append(held, c) // accept, then never answer
		}
	}()
	deaf := apitest.SocketPath(t)
	dl, err := transport.Listen(deaf) // listens, never accepts
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = dl.Close() })
	open, _ := io.Pipe() // input that never ends and never sends
	t.Cleanup(func() { _ = open.Close() })
	tests := []struct {
		name   string
		socket string
		stdin  io.Reader
	}{
		{"silent server", silent, strings.NewReader(`{"hook_event_name":"PreToolUse"}`)},
		{"server that never accepts", deaf, strings.NewReader(`{"hook_event_name":"PreToolUse"}`)},
		{"input never ends", silent, open},
		{"payload, then input never ends", silent, io.MultiReader(strings.NewReader(`{"hook_event_name":"Stop"}`), open)},
	}
	// Warm up: the first dial and the first goroutines cost more.
	_ = Hook{Pane: "7", Socket: silent, Version: "test"}.Run(strings.NewReader(`{"hook_event_name":"Stop"}`))
	for _, tt := range tests {
		start := time.Now()
		if err := (Hook{Pane: "7", Socket: tt.socket, Version: "test"}).Run(tt.stdin); err == nil {
			t.Errorf("%s: Run() = nil", tt.name)
		}
		if d := time.Since(start); d > sendBudget+250*time.Millisecond {
			t.Errorf("%s: Run() took %v, want about %v at most", tt.name, d, sendBudget)
		}
	}
}

// panicReader stands in for a failure deep in snow hook.
type panicReader struct{}

func (panicReader) Read([]byte) (int, error) { panic("boom") }

// TestHookPanicIsAnError: a panic while reading the input becomes an
// error, so snow hook can still exit 0.
func TestHookPanicIsAnError(t *testing.T) {
	if err := (Hook{Pane: "7", Socket: "/no/such.sock", Version: "test"}).Run(panicReader{}); err == nil {
		t.Fatal("Run() = nil after a panic")
	}
}

// TestStateForReadsOneValue: StateFor reads the event's name from the
// start of the input and never more than its cap, so a huge tool input
// cannot hold it, and it stops after the first JSON value.
func TestStateForReadsOneValue(t *testing.T) {
	huge := `{"session_id":"s","hook_event_name":"PreToolUse","tool_input":{"content":"` + strings.Repeat("x", 4<<20) + `"}}`
	r := &countReader{r: strings.NewReader(huge)}
	if state, change := StateFor(r); !change || state != api.AgentWorking {
		t.Fatalf("StateFor(4 MiB) = %q, %v; want working from the name before the cap", state, change)
	}
	if r.n > 1<<20 {
		t.Fatalf("StateFor read %d bytes, want no more than its cap", r.n)
	}
	state, change := StateFor(io.MultiReader(strings.NewReader(`{"hook_event_name":"Stop"} `), panicReader{}))
	if !change || state != api.AgentIdle {
		t.Fatalf("StateFor() = %q, %v; want idle without reading on", state, change)
	}
}

// countReader counts the bytes read through it.
type countReader struct {
	r io.Reader
	n int
}

func (c *countReader) Read(b []byte) (int, error) {
	n, err := c.r.Read(b)
	c.n += n
	return n, err
}
