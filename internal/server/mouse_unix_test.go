//go:build !windows

package server

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/pkg/api"
)

// waitMouse reads messages until a PaneUpdate for pane carries Mouse == want.
func waitMouse(t *testing.T, c *apitest.Client, pane int, want bool) {
	t.Helper()
	deadline := time.Now().Add(apitest.Wait)
	for time.Now().Before(deadline) {
		m, err := c.Next(time.Second)
		if err != nil && !errors.Is(err, apitest.ErrTimeout) {
			t.Fatal(err)
		}
		if u, ok := m.(*api.PaneUpdate); ok && u.Pane == pane && u.Mouse == want {
			return
		}
	}
	t.Fatalf("no update with mouse = %v", want)
}

// TestMouseReachesProgramsThatAskForIt: the pane reports its mouse mode in
// its updates, and a Mouse message becomes the bytes the program asked for.
// cat -v shows them as text.
func TestMouseReachesProgramsThatAskForIt(t *testing.T) {
	s := newTestServer(t)
	c, id := attached(t, s, 80, 24)
	c.Send(&api.Input{Pane: id, Paste: "printf '\\033[?1000h\\033[?1006h'; read x; printf '\\033[?1000l'\r"})
	waitMouse(t, c, id, true)

	c.Send(&api.Mouse{Pane: id, X: 3, Y: 1, Button: api.ButtonLeft, Action: api.ActionPress})
	c.WaitScreen("^[[<0;4;2M")
	c.Send(&api.Mouse{Pane: id, X: 3, Y: 1, Button: api.ButtonWheelDown, Action: api.ActionPress})
	c.WaitScreen("^[[<65;4;2M")

	c.Send(&api.Input{Pane: id, Paste: "\r"}) // read returns; the program turns the mouse off
	waitMouse(t, c, id, false)
}

// TestMouseIgnoredWithoutMouseMode: a shell that did not ask for the mouse
// receives nothing, and a bad event is ignored without closing the
// connection.
func TestMouseIgnoredWithoutMouseMode(t *testing.T) {
	s := newTestServer(t)
	c, id := attached(t, s, 80, 24)
	c.Send(&api.Mouse{Pane: id, X: 3, Y: 1, Button: api.ButtonLeft, Action: api.ActionPress})
	c.Send(&api.Mouse{Pane: id, X: -3, Y: 1, Button: api.ButtonLeft, Action: api.ActionPress})
	c.Send(&api.Mouse{Pane: 999, X: 3, Y: 1, Button: api.ButtonLeft, Action: api.ActionPress})
	c.Send(&api.Input{Pane: id, Paste: "echo after\r"})
	c.WaitScreen("after")
	if got := c.Screen(); strings.Contains(got, "[<0") {
		t.Fatalf("mouse bytes reached a shell that did not ask for them:\n%s", got)
	}
}
