package server

import (
	"strings"
	"testing"

	"github.com/emmyfong/snow/pkg/api"
)

func TestWindowCycle(t *testing.T) {
	s := newTestServer(t)
	c, _ := attached(t, s, 80, 24)
	for n := 2; n <= 3; n++ {
		c.Send(&api.Command{Op: api.OpNewWindow})
		nextLayout(t, c, func(l *api.Layout) bool { return len(l.Windows) == n && l.Active == n })
	}
	c.Send(&api.Command{Op: api.OpNextWindow})
	nextLayout(t, c, func(l *api.Layout) bool { return l.Active == 1 }) // wraps from 3
	c.Send(&api.Command{Op: api.OpPrevWindow})
	nextLayout(t, c, func(l *api.Layout) bool { return l.Active == 3 }) // wraps from 1
	c.Send(&api.Command{Op: api.OpSelectWindow, Target: 2})
	nextLayout(t, c, func(l *api.Layout) bool { return l.Active == 2 })
}

func TestRenameWindow(t *testing.T) {
	s := newTestServer(t)
	c, _ := attached(t, s, 80, 24)
	c.Send(&api.Command{Op: api.OpRenameWindow, Target: 1, Args: map[string]string{api.ArgName: "logs"}})
	nextLayout(t, c, func(l *api.Layout) bool { return l.Windows[0].Name == "logs" })
}

func TestNewWindowShowsItsScreen(t *testing.T) {
	s := newTestServer(t)
	c, _ := attached(t, s, 80, 24)
	c.Send(&api.Command{Op: api.OpNewWindow})
	l := nextLayout(t, c, func(l *api.Layout) bool { return l.Active == 2 })
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
	c.Send(&api.Command{Op: api.OpCloseWindow, Target: 2})
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(l.Windows) == 1 })
	if l.Active != 1 {
		t.Fatalf("shown window %d after closing 2, want 1", l.Active)
	}
}
