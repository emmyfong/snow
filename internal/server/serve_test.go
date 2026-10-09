package server

import (
	"errors"
	"io"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/pkg/api"
)

// testClient is a protocol client that rebuilds the screen from PaneUpdates.
// One goroutine reads the connection: Conn.Receive must never run in two
// goroutines at once, or frames interleave.
type testClient struct {
	t        *testing.T
	raw      net.Conn
	conn     *api.Conn
	incoming chan received
	rows     map[int]string
	updates  int
	lastLen  int // rows in the most recent PaneUpdate
}

type received struct {
	m   api.Message
	err error
}

// connect runs the handshake over an in-memory connection served by s.
func connect(t *testing.T, s *Server, cols, rows int) *testClient {
	t.Helper()
	a, b := net.Pipe()
	go s.serveConn(b)
	t.Cleanup(func() { _ = a.Close() })
	c := api.NewConn(a)
	if _, err := api.ClientHandshake(c, api.Hello{ClientVersion: "test", Protocol: api.ProtocolVersion, Cols: cols, Rows: rows}); err != nil {
		t.Fatalf("handshake: %v", err)
	}
	tc := &testClient{t: t, raw: a, conn: c, incoming: make(chan received, 1024), rows: map[int]string{}}
	go func() {
		for {
			m, err := c.Receive()
			tc.incoming <- received{m, err}
			if err != nil {
				return
			}
		}
	}()
	return tc
}

func (c *testClient) close() { _ = c.raw.Close() }

func (c *testClient) send(m api.Message) {
	c.t.Helper()
	if err := c.conn.Send(m); err != nil {
		c.t.Fatalf("send %T: %v", m, err)
	}
}

// next receives one message, applying PaneUpdates to the local screen.
func (c *testClient) next(timeout time.Duration) (api.Message, error) {
	select {
	case r := <-c.incoming:
		if u, ok := r.m.(*api.PaneUpdate); ok {
			c.updates++
			c.lastLen = len(u.Lines)
			for _, l := range u.Lines {
				c.rows[l.Row] = l.Text
			}
		}
		return r.m, r.err
	case <-time.After(timeout):
		return nil, errTimeout
	}
}

var errTimeout = errors.New("timed out")

// screen returns the rebuilt screen as plain text, one row per line, with
// each row's trailing padding removed.
func (c *testClient) screen() string {
	var b strings.Builder
	for i := range 200 {
		if l, ok := c.rows[i]; ok {
			b.WriteString(strings.TrimRight(stripANSI(l), " "))
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// waitScreen reads messages until the rebuilt screen contains want.
func (c *testClient) waitScreen(want string) {
	c.t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(c.screen(), want) {
			return
		}
		if _, err := c.next(time.Second); err != nil && !errors.Is(err, errTimeout) {
			c.t.Fatalf("waiting for %q: %v; screen:\n%s", want, err, c.screen())
		}
	}
	c.t.Fatalf("screen never showed %q:\n%s", want, c.screen())
}

// drain reads until no message arrives for quiet.
func (c *testClient) drain(quiet time.Duration) {
	for {
		if _, err := c.next(quiet); err != nil {
			return
		}
	}
}

func TestAttachSendsScreen(t *testing.T) {
	s := newTestServer(t)
	c := connect(t, s, 80, 24)
	c.send(&api.Attach{Session: "work", Create: true})
	m, err := c.next(5 * time.Second)
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
	c.waitScreen(strings.TrimRight(prompt(), " "))
}

func TestAttachMissingSession(t *testing.T) {
	s := newTestServer(t)
	c := connect(t, s, 80, 24)
	c.send(&api.Attach{Session: "nope"})
	m, err := c.next(5 * time.Second)
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
	c.send(&api.Attach{Session: "work", Create: true})
	c.waitScreen(strings.TrimRight(prompt(), " "))
	c.drain(300 * time.Millisecond)
	typeLine(s.pane(t, "work"), "echo snowmark")
	c.waitScreen("snowmark\n")
	if c.lastLen >= 24 {
		t.Fatalf("an update after the first screen carried %d rows; want only changed rows", c.lastLen)
	}
}

func TestUpdateRateCapped(t *testing.T) {
	opts := testOptions()
	opts.Spec = floodSpec
	s := New(opts)
	t.Cleanup(s.Close)
	c := connect(t, s, 80, 24)
	c.send(&api.Attach{Session: "flood", Create: true})
	c.waitScreen("x")
	c.updates = 0
	start := time.Now()
	for time.Since(start) < time.Second {
		if _, err := c.next(time.Second); err != nil {
			t.Fatal(err)
		}
	}
	if c.updates > 35 {
		t.Fatalf("%d updates in one second under a flood of output, want at most about 30", c.updates)
	}
}

func TestSessionEndClosesConnection(t *testing.T) {
	s := newTestServer(t)
	c := connect(t, s, 80, 24)
	c.send(&api.Attach{Session: "work", Create: true})
	c.waitScreen(strings.TrimRight(prompt(), " "))
	typeLine(s.pane(t, "work"), "exit")
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		_, err := c.next(time.Second)
		if errors.Is(err, io.EOF) || (err != nil && !errors.Is(err, errTimeout)) {
			return
		}
	}
	t.Fatal("connection stayed open after the session ended")
}

// floodSpec prints x forever.
func floodSpec() (spec termSpec, profile string) {
	if runtime.GOOS == "windows" {
		return termSpec{Path: "cmd.exe", Args: []string{"/c", "for /l %i in () do @echo x"}}, "flood"
	}
	return termSpec{Path: "/bin/sh", Args: []string{"-c", "while :; do echo x; done"}}, "flood"
}

func TestInputReachesShell(t *testing.T) {
	s := newTestServer(t)
	c := connect(t, s, 120, 24)
	c.send(&api.Attach{Session: "work", Create: true})
	c.waitScreen(strings.TrimRight(prompt(), " "))
	for _, r := range "echo typed" {
		code := string(r)
		if r == ' ' {
			code = api.KeySpace // the protocol names space; " " is invalid
		}
		c.send(&api.Input{Pane: 1, Key: &api.Key{Code: code, Text: string(r)}})
	}
	c.send(&api.Input{Pane: 1, Key: &api.Key{Code: api.KeyEnter}})
	c.waitScreen("\ntyped\n")
}

func TestPasteReachesShell(t *testing.T) {
	s := newTestServer(t)
	c := connect(t, s, 120, 24)
	c.send(&api.Attach{Session: "work", Create: true})
	c.waitScreen(strings.TrimRight(prompt(), " "))
	c.send(&api.Input{Pane: 1, Paste: "echo pasted\r"})
	c.waitScreen("\npasted\n")
}

func TestInputForOtherSessionsPaneIgnored(t *testing.T) {
	s := newTestServer(t)
	other := connect(t, s, 80, 24)
	other.send(&api.Attach{Session: "other", Create: true})
	other.waitScreen(strings.TrimRight(prompt(), " "))
	c := connect(t, s, 80, 24)
	c.send(&api.Attach{Session: "mine", Create: true})
	c.waitScreen(strings.TrimRight(prompt(), " "))
	// Pane 1 belongs to "other": a client attached to "mine" must not reach it.
	c.send(&api.Input{Pane: 1, Paste: "echo intruder\r"})
	time.Sleep(500 * time.Millisecond)
	other.drain(300 * time.Millisecond)
	if strings.Contains(other.screen(), "intruder") {
		t.Fatal("a client typed into a pane of a session it is not attached to")
	}
}
