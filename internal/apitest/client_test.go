package apitest

import (
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/pkg/api"
)

// fakeServer answers the handshake on b and then sends msgs.
func fakeServer(t *testing.T, b net.Conn, msgs ...api.Message) {
	go func() {
		conn := api.NewConn(b)
		if _, err := api.ServerHandshake(conn, api.Welcome{ServerVersion: "fake", Protocol: api.ProtocolVersion}); err != nil {
			t.Errorf("handshake: %v", err)
			return
		}
		for _, m := range msgs {
			if err := conn.Send(m); err != nil {
				return
			}
		}
	}()
}

func TestClientRebuildsScreen(t *testing.T) {
	a, b := net.Pipe()
	fakeServer(t, b,
		&api.PaneUpdate{Pane: 1, Lines: []api.LineUpdate{{Row: 0, Text: "first   "}, {Row: 1, Text: "\x1b[1mbold\x1b[m"}}},
		&api.PaneUpdate{Pane: 1, Lines: []api.LineUpdate{{Row: 0, Text: "changed"}}},
	)
	c := New(t, a, 80, 24)
	c.WaitScreen("changed")
	if got, want := c.Screen(), "changed\nbold\n"; got != want {
		t.Fatalf("Screen() = %q, want %q", got, want)
	}
	if c.Updates != 2 || c.LastLen != 1 {
		t.Fatalf("Updates, LastLen = %d, %d; want 2, 1", c.Updates, c.LastLen)
	}
}

func TestNextTimesOut(t *testing.T) {
	a, b := net.Pipe()
	fakeServer(t, b)
	c := New(t, a, 80, 24)
	if _, err := c.Next(50 * time.Millisecond); !errors.Is(err, ErrTimeout) {
		t.Fatalf("Next() error = %v, want ErrTimeout", err)
	}
}

func TestSocketPathIsShort(t *testing.T) {
	if p := SocketPath(t); len(p) > 100 || !strings.HasSuffix(p, "default.sock") {
		t.Fatalf("SocketPath() = %q", p)
	}
}
