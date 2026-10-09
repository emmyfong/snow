package server

import (
	"errors"
	"io"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/pkg/api"
)

// connect runs the handshake over an in-memory connection served by s.
func connect(t *testing.T, s *Server, cols, rows int) *apitest.Client {
	t.Helper()
	a, b := net.Pipe()
	go s.serveConn(b)
	return apitest.New(t, a, cols, rows)
}

func TestAttachSendsScreen(t *testing.T) {
	s := newTestServer(t)
	c := connect(t, s, 80, 24)
	c.Send(&api.Attach{Session: "work", Create: true})
	m, err := c.Next(5 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	l, ok := m.(*api.Layout)
	if !ok || l.Session != "work" || len(l.Windows) != 1 || len(l.Windows[0].Panes) != 1 {
		t.Fatalf("first message %#v, want a Layout with one window and one pane", m)
	}
	if r := l.Windows[0].Panes[0].Rect; r.W != 80 || r.H != 24 {
		t.Fatalf("pane rect %+v, want 80x24", r)
	}
	c.WaitScreen(strings.TrimRight(prompt(), " "))
}

func TestAttachMissingSession(t *testing.T) {
	s := newTestServer(t)
	c := connect(t, s, 80, 24)
	c.Send(&api.Attach{Session: "nope"})
	m, err := c.Next(5 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if e, ok := m.(*api.Error); !ok || e.Code != api.CodeNoSession {
		t.Fatalf("got %#v, want Error{%s}", m, api.CodeNoSession)
	}
}

func TestChangedLinesOnly(t *testing.T) {
	s := newTestServer(t)
	c := connect(t, s, 80, 24)
	c.Send(&api.Attach{Session: "work", Create: true})
	c.WaitScreen(strings.TrimRight(prompt(), " "))
	c.Drain(300 * time.Millisecond)
	typeLine(s.pane(t, "work"), "echo snowmark")
	c.WaitScreen("snowmark\n")
	if c.LastLen >= 24 {
		t.Fatalf("an update after the first screen carried %d rows; want only changed rows", c.LastLen)
	}
}

func TestUpdateRateCapped(t *testing.T) {
	opts := testOptions()
	opts.Spec = floodSpec
	s := New(opts)
	t.Cleanup(s.Close)
	c := connect(t, s, 80, 24)
	c.Send(&api.Attach{Session: "flood", Create: true})
	c.WaitScreen("x")
	c.Updates = 0
	start := time.Now()
	for time.Since(start) < time.Second {
		if _, err := c.Next(time.Second); err != nil {
			t.Fatal(err)
		}
	}
	if c.Updates > 35 {
		t.Fatalf("%d updates in one second under a flood of output, want at most about 30", c.Updates)
	}
}

func TestSessionEndClosesConnection(t *testing.T) {
	s := newTestServer(t)
	c := connect(t, s, 80, 24)
	c.Send(&api.Attach{Session: "work", Create: true})
	c.WaitScreen(strings.TrimRight(prompt(), " "))
	typeLine(s.pane(t, "work"), "exit")
	ended := false
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		m, err := c.Next(time.Second)
		if e, ok := m.(*api.Error); ok && e.Code == api.CodeSessionEnded {
			ended = true
		}
		if errors.Is(err, io.EOF) || (err != nil && !errors.Is(err, apitest.ErrTimeout)) {
			if !ended {
				t.Fatal("connection closed without saying the session ended")
			}
			return
		}
	}
	t.Fatal("connection stayed open after the session ended")
}

// floodSpec prints x forever.
func floodSpec(string) (spec term.Spec, profile string) {
	if runtime.GOOS == "windows" {
		return term.Spec{Path: "cmd.exe", Args: []string{"/c", "for /l %i in () do @echo x"}}, "flood"
	}
	return term.Spec{Path: "/bin/sh", Args: []string{"-c", "while :; do echo x; done"}}, "flood"
}

func TestInputReachesShell(t *testing.T) {
	s := newTestServer(t)
	c := connect(t, s, 120, 24)
	c.Send(&api.Attach{Session: "work", Create: true})
	c.WaitScreen(strings.TrimRight(prompt(), " "))
	for _, r := range "echo typed" {
		code := string(r)
		if r == ' ' {
			code = api.KeySpace // the protocol names space; " " is invalid
		}
		c.Send(&api.Input{Pane: 1, Key: &api.Key{Code: code, Text: string(r)}})
	}
	c.Send(&api.Input{Pane: 1, Key: &api.Key{Code: api.KeyEnter}})
	c.WaitScreen("\ntyped\n")
}

func TestPasteReachesShell(t *testing.T) {
	s := newTestServer(t)
	c := connect(t, s, 120, 24)
	c.Send(&api.Attach{Session: "work", Create: true})
	c.WaitScreen(strings.TrimRight(prompt(), " "))
	c.Send(&api.Input{Pane: 1, Paste: "echo pasted\r"})
	c.WaitScreen("\npasted\n")
}

func TestInputForOtherSessionsPaneIgnored(t *testing.T) {
	s := newTestServer(t)
	other := connect(t, s, 80, 24)
	other.Send(&api.Attach{Session: "other", Create: true})
	other.WaitScreen(strings.TrimRight(prompt(), " "))
	c := connect(t, s, 80, 24)
	c.Send(&api.Attach{Session: "mine", Create: true})
	c.WaitScreen(strings.TrimRight(prompt(), " "))
	// Pane 1 belongs to "other": a client attached to "mine" must not reach it.
	c.Send(&api.Input{Pane: 1, Paste: "echo intruder\r"})
	// The server handles one connection's input in order, so once c's own
	// pane shows this, the intruding input was handled too.
	c.Send(&api.Input{Pane: 2, Paste: "echo mine\r"})
	c.WaitScreen("\nmine\n")
	other.Drain(300 * time.Millisecond)
	if strings.Contains(other.Screen(), "intruder") {
		t.Fatal("a client typed into a pane of a session it is not attached to")
	}
}

func TestOversizeResizeIgnored(t *testing.T) {
	s := newTestServer(t)
	c := connect(t, s, 80, 24)
	c.Send(&api.Attach{Session: "work", Create: true})
	c.WaitScreen(strings.TrimRight(prompt(), " "))
	c.Send(&api.Resize{Cols: 1 << 40, Rows: 3})
	for {
		m, err := c.Next(5 * time.Second)
		if err != nil {
			t.Fatalf("no reply to an oversize Resize: %v", err)
		}
		if e, ok := m.(*api.Error); ok {
			if e.Code != api.CodeBadSize {
				t.Fatalf("error %q, want %s", e.Code, api.CodeBadSize)
			}
			break
		}
	}
	if cols, rows := s.pane(t, "work").Size(); cols != 80 || rows != 24 {
		t.Fatalf("pane is %dx%d after an oversize Resize, want 80x24 kept", cols, rows)
	}
	if len(s.Sessions()) != 1 {
		t.Fatal("server lost its session")
	}
}

func TestOversizeHelloUsesDefaultSize(t *testing.T) {
	s := newTestServer(t)
	c := connect(t, s, 100000, 100000)
	c.Send(&api.Attach{Session: "big", Create: true})
	c.WaitScreen(strings.TrimRight(prompt(), " "))
	if cols, rows := s.pane(t, "big").Size(); cols > api.MaxCols || rows > api.MaxRows {
		t.Fatalf("pane is %dx%d, want within %dx%d", cols, rows, api.MaxCols, api.MaxRows)
	}
}
