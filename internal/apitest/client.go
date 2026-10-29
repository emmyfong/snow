package apitest

import (
	"errors"
	"net"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/transport"
	"github.com/emmyfong/snow/pkg/api"
)

// Wait is how long WaitScreen and Attach wait before they fail the test.
const Wait = 15 * time.Second

// ErrTimeout is returned by Next when no message arrives in time.
var ErrTimeout = errors.New("apitest: timed out")

// Client is a protocol client that rebuilds the screen from PaneUpdates.
// One goroutine reads the connection, because Conn.Receive must never run
// in two goroutines at once. Use a Client from one test goroutine.
type Client struct {
	t        testing.TB
	raw      net.Conn
	conn     *api.Conn
	incoming chan received
	rows     map[int]string
	chats    map[int]*api.ChatSnapshot // pane ID → rebuilt chat
	agents   *api.AgentList            // the last agent list

	// Updates counts PaneUpdates received; LastLen is the number of rows
	// in the most recent one.
	Updates, LastLen int
	// Snapshots counts ChatSnapshots received.
	Snapshots int
}

type received struct {
	m   api.Message
	err error
}

// New runs the handshake over raw with the given terminal size. The
// connection closes when the test ends.
func New(t testing.TB, raw net.Conn, cols, rows int) *Client {
	t.Helper()
	t.Cleanup(func() { _ = raw.Close() })
	conn := api.NewConn(raw)
	hello := api.Hello{ClientVersion: "test", Protocol: api.ProtocolVersion, Cols: cols, Rows: rows}
	if _, err := api.ClientHandshake(conn, hello); err != nil {
		t.Fatalf("handshake: %v", err)
	}
	c := &Client{t: t, raw: raw, conn: conn, incoming: make(chan received, 1024), rows: map[int]string{}, chats: map[int]*api.ChatSnapshot{}}
	go func() {
		for {
			m, err := conn.Receive()
			c.incoming <- received{m, err}
			if err != nil {
				return
			}
		}
	}()
	return c
}

// Dial connects to the server at path, retrying while it starts.
func Dial(t testing.TB, path string, cols, rows int) *Client {
	t.Helper()
	deadline := time.Now().Add(Wait)
	for {
		raw, err := transport.Dial(path)
		if err == nil {
			return New(t, raw, cols, rows)
		}
		if time.Now().After(deadline) {
			t.Fatalf("dial %s: %v", path, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// Close closes the connection.
func (c *Client) Close() { _ = c.raw.Close() }

// Send sends m or fails the test.
func (c *Client) Send(m api.Message) {
	c.t.Helper()
	if err := c.conn.Send(m); err != nil {
		c.t.Fatalf("send %T: %v", m, err)
	}
}

// Next receives one message, applying a PaneUpdate to the screen and a
// ChatSnapshot or ChatEvent to its chat. It returns ErrTimeout if nothing
// arrives within timeout.
func (c *Client) Next(timeout time.Duration) (api.Message, error) {
	select {
	case r := <-c.incoming:
		switch m := r.m.(type) {
		case *api.PaneUpdate:
			c.Updates++
			c.LastLen = len(m.Lines)
			for _, l := range m.Lines {
				c.rows[l.Row] = l.Text
			}
		case *api.ChatSnapshot:
			snap := *m
			c.chats[m.Pane] = &snap
			c.Snapshots++
		case *api.ChatEvent:
			if s, ok := c.chats[m.Pane]; ok {
				s.Apply(m)
			}
		case *api.AgentList:
			c.agents = m
		}
		return r.m, r.err
	case <-time.After(timeout):
		return nil, ErrTimeout
	}
}

// Attach attaches to an existing session, retrying while it does not exist
// yet, for example because another client is still creating it.
func (c *Client) Attach(session string) {
	c.t.Helper()
	deadline := time.Now().Add(Wait)
	c.Send(&api.Attach{Session: session})
	for time.Now().Before(deadline) {
		m, err := c.Next(time.Second)
		switch m := m.(type) {
		case *api.Layout:
			return
		case *api.Error:
			if m.Code != api.CodeNoSession {
				c.t.Fatalf("attach %s: %v", session, m)
			}
			time.Sleep(20 * time.Millisecond)
			c.Send(&api.Attach{Session: session})
			continue
		}
		if err != nil && !errors.Is(err, ErrTimeout) {
			c.t.Fatalf("attach %s: %v", session, err)
		}
	}
	c.t.Fatalf("session %s never appeared", session)
}

// Screen returns the rebuilt screen as plain text, one row per line, with
// each row's trailing padding removed.
func (c *Client) Screen() string {
	var b strings.Builder
	for i := range api.MaxRows {
		if l, ok := c.rows[i]; ok {
			b.WriteString(strings.TrimRight(ansi.Strip(l), " "))
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// WaitScreen reads messages until the screen contains want.
func (c *Client) WaitScreen(want string) {
	c.t.Helper()
	deadline := time.Now().Add(Wait)
	for time.Now().Before(deadline) {
		if strings.Contains(c.Screen(), want) {
			return
		}
		if _, err := c.Next(100 * time.Millisecond); err != nil && !errors.Is(err, ErrTimeout) {
			c.t.Fatalf("waiting for %q: %v; screen:\n%s", want, err, c.Screen())
		}
	}
	c.t.Fatalf("screen never showed %q:\n%s", want, c.Screen())
}

// Chat returns the rebuilt chat of pane, or nil before its first
// snapshot.
func (c *Client) Chat(pane int) *api.ChatSnapshot { return c.chats[pane] }

// Chats returns the IDs of the chat panes with a snapshot.
func (c *Client) Chats() []int {
	ids := make([]int, 0, len(c.chats))
	for id := range c.chats {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// WaitChat reads messages until ok reports true for pane's chat, and
// returns the chat.
func (c *Client) WaitChat(pane int, ok func(*api.ChatSnapshot) bool) *api.ChatSnapshot {
	c.t.Helper()
	deadline := time.Now().Add(Wait)
	for time.Now().Before(deadline) {
		if s := c.chats[pane]; s != nil && ok(s) {
			return s
		}
		if _, err := c.Next(100 * time.Millisecond); err != nil && !errors.Is(err, ErrTimeout) {
			c.t.Fatalf("waiting for chat %d: %v", pane, err)
		}
	}
	c.t.Fatalf("chat %d never got there: %+v", pane, c.chats[pane])
	return nil
}

// ChatText returns the text of pane's transcript, one entry per line, as
// kind: text, for matching in tests.
func (c *Client) ChatText(pane int) string {
	s := c.chats[pane]
	if s == nil {
		return ""
	}
	var b strings.Builder
	for _, e := range s.Entries {
		b.WriteString(e.Kind + ": " + e.Text)
		if e.Tool != nil {
			b.WriteString(e.Tool.Name + " " + e.Tool.Result)
		}
		if e.Error != nil {
			b.WriteString(e.Error.Code)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// Drain reads messages until none arrives for quiet.
func (c *Client) Drain(quiet time.Duration) {
	for {
		if _, err := c.Next(quiet); err != nil {
			return
		}
	}
}

// Reply reads c until a message of type T or an Error arrives, and returns
// whichever came first; other messages, such as the session lists every
// connection gets, are skipped. It fails the test after Wait.
func Reply[T api.Message](c *Client) (T, *api.Error) {
	c.t.Helper()
	var zero T
	deadline := time.Now().Add(Wait)
	for time.Now().Before(deadline) {
		m, err := c.Next(time.Second)
		if errors.Is(err, ErrTimeout) {
			continue
		}
		if err != nil {
			c.t.Fatalf("waiting for %T: %v", zero, err)
		}
		switch m := m.(type) {
		case T:
			return m, nil
		case *api.Error:
			return zero, m
		}
	}
	c.t.Fatalf("no %T in %v", zero, Wait)
	return zero, nil
}

// Agents returns the last agent list, or nil before the first.
func (c *Client) Agents() []api.AgentInfo {
	if c.agents == nil {
		return nil
	}
	return c.agents.Agents
}

// WaitAgents reads messages until ok reports true for the last agent
// list, and returns that list. It asks ok about the list it already has
// first, then about each new one in order.
func (c *Client) WaitAgents(ok func([]api.AgentInfo) bool) []api.AgentInfo {
	c.t.Helper()
	if c.agents != nil && ok(c.agents.Agents) {
		return c.agents.Agents
	}
	deadline := time.Now().Add(Wait)
	for time.Now().Before(deadline) {
		m, err := c.Next(100 * time.Millisecond)
		if err != nil && !errors.Is(err, ErrTimeout) {
			c.t.Fatalf("waiting for an agent list: %v", err)
		}
		if l, is := m.(*api.AgentList); is && ok(l.Agents) {
			return l.Agents
		}
	}
	c.t.Fatalf("no matching agent list; last: %+v", c.Agents())
	return nil
}
