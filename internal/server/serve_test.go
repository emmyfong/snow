package server

import (
	"net"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/pkg/api"
)

// connect runs the handshake over an in-memory connection served by s and
// reads the session list and the agent list the server sends every new
// client, so a test's next message is the answer to its own request. The
// client keeps the agent list (apitest.Client.Agents).
func connect(t testing.TB, s *Server, cols, rows int) *apitest.Client {
	t.Helper()
	a, b := net.Pipe()
	go s.serveConn(b)
	c := apitest.New(t, a, cols, rows)
	m, err := c.Next(5 * time.Second)
	if _, ok := m.(*api.SessionList); !ok {
		t.Fatalf("first message %#v (%v), want the session list", m, err)
	}
	if m, err := c.Next(5 * time.Second); !isAgentList(m) {
		t.Fatalf("second message %#v (%v), want the agent list", m, err)
	}
	return c
}

func isAgentList(m api.Message) bool { _, ok := m.(*api.AgentList); return ok }

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
	if r := l.Windows[0].Panes[0].Rect; r.W != 80 || r.H != 23 {
		t.Fatalf("pane rect %+v, want 80x23 (one row for the status bar)", r)
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

// TestSessionEndDetachesClient: when the last pane exits, the attached
// client hears that the session ended and stays connected, unattached, so it
// can show the home page.
func TestSessionEndDetachesClient(t *testing.T) {
	s := newTestServer(t)
	c := connect(t, s, 80, 24)
	c.Send(&api.Attach{Session: "work", Create: true})
	c.WaitScreen(strings.TrimRight(prompt(), " "))
	typeLine(s.pane(t, "work"), "exit")
	if e := nextError(t, c); e.Code != api.CodeSessionEnded {
		t.Fatalf("got %+v, want CodeSessionEnded", e)
	}
	nextList(t, c, func(l *api.SessionList) bool {
		return !slices.ContainsFunc(l.Sessions, func(i api.SessionInfo) bool { return i.Name == "work" })
	})
	// Still connected: the server answers a request.
	c.Send(&api.KillSession{Session: "nope"})
	if e := nextError(t, c); e.Code != api.CodeCommandFailed {
		t.Fatalf("after the session ended: %+v, want CodeCommandFailed from a live connection", e)
	}
}

// floodSpec prints x forever.
func floodSpec(_, _ string) (spec term.Spec, profile string, err error) {
	if runtime.GOOS == "windows" {
		return term.Spec{Path: "cmd.exe", Args: []string{"/c", "for /l %i in () do @echo x"}}, "flood", nil
	}
	return term.Spec{Path: "/bin/sh", Args: []string{"-c", "while :; do echo x; done"}}, "flood", nil
}

// typeInput types text and Enter into pane 1 as key presses in the shape the
// client sends: an uppercase letter is its lowercase code with Shift.
func typeInput(c *apitest.Client, text string) {
	for _, r := range text {
		k := api.Key{Code: string(r), Text: string(r)}
		switch {
		case r == ' ':
			k.Code = api.KeySpace // the protocol names space; " " is invalid
		case unicode.IsUpper(r):
			k.Code, k.Mod = string(unicode.ToLower(r)), []string{api.ModShift}
		}
		c.Send(&api.Input{Pane: 1, Key: &k})
	}
	c.Send(&api.Input{Pane: 1, Key: &api.Key{Code: api.KeyEnter}})
}

func TestInputReachesShell(t *testing.T) {
	s := newTestServer(t)
	c := connect(t, s, 120, 24)
	c.Send(&api.Attach{Session: "work", Create: true})
	c.WaitScreen(strings.TrimRight(prompt(), " "))
	typeInput(c, "echo typed")
	c.WaitScreen("\ntyped\n")
}

// TestUppercaseReachesShell: Shift+letter reached the shell as nothing, so
// "echo ABC" ran as "echo " (#236).
func TestUppercaseReachesShell(t *testing.T) {
	s := newTestServer(t)
	c := connect(t, s, 120, 24)
	c.Send(&api.Attach{Session: "work", Create: true})
	c.WaitScreen(strings.TrimRight(prompt(), " "))
	typeInput(c, "echo ABC")
	c.WaitScreen("\nABC\n")
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
	if cols, rows := s.pane(t, "work").Size(); cols != 80 || rows != 23 {
		t.Fatalf("pane is %dx%d after an oversize Resize, want 80x23 kept", cols, rows)
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
