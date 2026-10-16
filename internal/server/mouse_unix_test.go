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
// its updates, and each Mouse message becomes the bytes the program asked
// for. The tty is raw, so cat -v shows what the program really received.
// internal/term covers the encodings and runs on Windows too.
func TestMouseReachesProgramsThatAskForIt(t *testing.T) {
	s := newTestServer(t)
	c, id := attached(t, s, 80, 24)
	c.Send(&api.Input{Pane: id, Paste: "printf '\\033[?1002h\\033[?1006h'; stty raw -echo; cat -v\r"})
	waitMouse(t, c, id, true)

	for _, step := range []struct {
		ev   api.Mouse
		want string
	}{
		{api.Mouse{Pane: id, X: 3, Y: 1, Button: api.ButtonLeft, Action: api.ActionPress}, "^[[<0;4;2M"},
		{api.Mouse{Pane: id, X: 5, Y: 1, Button: api.ButtonLeft, Action: api.ActionMotion}, "^[[<32;6;2M"},
		{api.Mouse{Pane: id, X: 5, Y: 1, Button: api.ButtonLeft, Action: api.ActionRelease}, "^[[<0;6;2m"},
		{api.Mouse{Pane: id, X: 3, Y: 1, Button: api.ButtonWheelDown, Action: api.ActionPress}, "^[[<65;4;2M"},
	} {
		ev := step.ev
		c.Send(&ev)
		c.WaitScreen(step.want)
	}
	// Hover is not part of mode 1002.
	c.Send(&api.Mouse{Pane: id, X: 9, Y: 9, Button: api.ButtonNone, Action: api.ActionMotion})
	c.Send(&api.Input{Pane: id, Paste: "Z"})
	c.WaitScreen("^[[<65;4;2MZ")
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
