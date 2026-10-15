package server

import (
	"strings"
	"testing"

	"github.com/emmyfong/snow/pkg/api"
)

func TestWindowCycle(t *testing.T) {
	s := newTestServer(t)
	c, _ := attached(t, s, 80, 24)
	// Windows count from 0, as in tmux.
	for n := 1; n <= 2; n++ {
		c.Send(&api.Command{Op: api.OpNewWindow})
		nextLayout(t, c, func(l *api.Layout) bool { return len(l.Windows) == n+1 && l.Active == n })
	}
	c.Send(&api.Command{Op: api.OpNextWindow})
	nextLayout(t, c, func(l *api.Layout) bool { return l.Active == 0 }) // wraps from 2
	c.Send(&api.Command{Op: api.OpPrevWindow})
	nextLayout(t, c, func(l *api.Layout) bool { return l.Active == 2 }) // wraps from 0
	c.Send(&api.Command{Op: api.OpSelectWindow, Target: 1})
	nextLayout(t, c, func(l *api.Layout) bool { return l.Active == 1 })
}

func TestRenameWindow(t *testing.T) {
	s := newTestServer(t)
	c, _ := attached(t, s, 80, 24)
	c.Send(&api.Command{Op: api.OpRenameWindow, Target: 0, Args: map[string]string{api.ArgName: "logs"}})
	nextLayout(t, c, func(l *api.Layout) bool { return l.Windows[0].Name == "logs" })
}

func TestNewWindowShowsItsScreen(t *testing.T) {
	s := newTestServer(t)
	c, _ := attached(t, s, 80, 24)
	c.Send(&api.Command{Op: api.OpNewWindow})
	l := nextLayout(t, c, func(l *api.Layout) bool { return l.Active == 1 })
	id := shownPanes(l)[0].ID
	eventually(t, "the new window's prompt", func() bool {
		return screenHas(s.pane(t, "work"), strings.TrimRight(prompt(), " "))
	})
	c.Send(&api.Input{Pane: id, Paste: "echo inwindow2\r"})
	c.WaitScreen("inwindow2")
}

func TestCloseWindow(t *testing.T) {
	s := newTestServer(t)
	c, _ := attached(t, s, 80, 24)
	c.Send(&api.Command{Op: api.OpNewWindow})
	nextLayout(t, c, func(l *api.Layout) bool { return len(l.Windows) == 2 })
	c.Send(&api.Command{Op: api.OpCloseWindow, Target: 1})
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(l.Windows) == 1 })
	if l.Active != 0 {
		t.Fatalf("shown window %d after closing 1, want 0", l.Active)
	}
}
