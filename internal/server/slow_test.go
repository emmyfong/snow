package server

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/pkg/api"
)

// silentClient attaches over an in-memory connection and then reads nothing
// until the test calls Receive, like a client whose terminal is paused.
func silentClient(t *testing.T, s *Server, session string) *api.Conn {
	t.Helper()
	a, b := net.Pipe()
	go s.serveConn(b)
	t.Cleanup(func() { _ = a.Close() })
	conn := api.NewConn(a)
	if _, err := api.ClientHandshake(conn, api.Hello{ClientVersion: "test", Protocol: api.ProtocolVersion, Cols: 80, Rows: 24}); err != nil {
		t.Fatal(err)
	}
	if err := conn.Send(&api.Attach{Session: session, Create: true}); err != nil {
		t.Fatal(err)
	}
	return conn
}

// TestSlowClientSkipsFrames: a client that stops reading during heavy output
// falls behind and catches up later; it is not disconnected.
func TestSlowClientSkipsFrames(t *testing.T) {
	opts := testOptions()
	opts.Spec = floodSpec
	s := New(opts)
	t.Cleanup(s.Close)
	s.outbox = 8
	conn := silentClient(t, s, "flood")
	time.Sleep(2 * time.Second) // about 60 frames: far more than the outbox

	if n := s.clientCount(); n != 1 {
		t.Fatalf("%d clients after a pause, want the slow client kept", n)
	}
	updates := 0
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		m, err := conn.Receive()
		if err != nil {
			t.Fatalf("slow client disconnected: %v", err)
		}
		if _, ok := m.(*api.PaneUpdate); ok {
			updates++
		}
	}
	if updates < 5 {
		t.Fatalf("%d updates in the second after catching up, want updates to keep coming", updates)
	}
}

// TestDroppedClientIsDisconnected: when control messages overflow a client
// that does not read, the server closes the connection at once, so the
// client can no longer type into the session.
func TestDroppedClientIsDisconnected(t *testing.T) {
	s := newTestServer(t)
	s.outbox = 4
	watch := connect(t, s, 80, 24)
	watch.Send(&api.Attach{Session: "work", Create: true})
	watch.WaitScreen(strings.TrimRight(prompt(), " "))

	conn := silentClient(t, s, "work")
	// Each size change sends every client of the session a Layout.
	for i := range 20 {
		if err := conn.Send(&api.Resize{Cols: 60 + i%2, Rows: 20}); err != nil {
			break
		}
	}
	eventually(t, "the dropped client's connection to close", func() bool {
		return conn.Send(&api.Input{Pane: 1, Paste: "echo leaked\r"}) != nil
	})
	watch.Drain(500 * time.Millisecond)
	if strings.Contains(watch.Screen(), "leaked") {
		t.Fatal("a dropped client still typed into the session")
	}
}
