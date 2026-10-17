package server

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/pkg/api"
)

// attached returns a client attached to a new session at the shell prompt,
// and the session's first pane ID.
func attached(t *testing.T, s *Server, cols, rows int) (*apitest.Client, int) {
	t.Helper()
	c := connect(t, s, cols, rows)
	c.Send(&api.Attach{Session: "work", Create: true})
	l := nextLayout(t, c, func(*api.Layout) bool { return true })
	c.WaitScreen(strings.TrimRight(prompt(), " "))
	return c, l.Windows[0].Panes[0].ID
}

// nextLayout reads until a Layout that satisfies ok arrives.
func nextLayout(t testing.TB, c *apitest.Client, ok func(*api.Layout) bool) *api.Layout {
	t.Helper()
	deadline := time.Now().Add(apitest.Wait)
	for time.Now().Before(deadline) {
		m, err := c.Next(time.Second)
		if err != nil && !errors.Is(err, apitest.ErrTimeout) {
			t.Fatal(err)
		}
		if e, isErr := m.(*api.Error); isErr {
			t.Fatalf("server error: %v", e)
		}
		if l, isLayout := m.(*api.Layout); isLayout && ok(l) {
			return l
		}
	}
	t.Fatal("no matching Layout")
	return nil
}

func shownPanes(l *api.Layout) []api.PaneInfo {
	for _, w := range l.Windows {
		if w.Index == l.Active {
			return w.Panes
		}
	}
	return nil
}

func split(id int, dir string) *api.Command {
	return &api.Command{Op: api.OpSplit, Target: id, Args: map[string]string{api.ArgDir: dir}}
}

func TestSplitStartsPane(t *testing.T) {
	s := newTestServer(t)
	c, first := attached(t, s, 81, 25)
	c.Send(split(first, api.DirRight))
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	panes := shownPanes(l)
	if panes[0].Rect != (api.Rect{X: 0, Y: 0, W: 40, H: 24}) || panes[1].Rect != (api.Rect{X: 41, Y: 0, W: 40, H: 24}) {
		t.Fatalf("rects %+v, %+v; want two 40x24 panes beside a border", panes[0].Rect, panes[1].Rect)
	}
	if !panes[1].Focused || panes[0].Focused {
		t.Fatal("the new pane should have the focus")
	}
	// The second shell runs: once its own prompt shows (shells drop input
	// while they start), typing into it shows output.
	eventually(t, "the new pane's prompt", func() bool {
		return screenHas(s.pane(t, "work"), strings.TrimRight(prompt(), " "))
	})
	c.Send(&api.Input{Pane: panes[1].ID, Paste: "echo second\r"})
	c.WaitScreen("second")
	if cols, rows := s.pane(t, "work").Size(); cols != 40 || rows != 24 {
		t.Fatalf("new pane's terminal is %dx%d, want 40x24", cols, rows)
	}
}

func TestSplitTooSmallFails(t *testing.T) {
	s := newTestServer(t)
	c, first := attached(t, s, 2, 5)
	c.Send(split(first, api.DirRight))
	for {
		m, err := c.Next(5 * time.Second)
		if err != nil {
			t.Fatalf("no answer to a split with no room: %v", err)
		}
		if e, ok := m.(*api.Error); ok {
			if e.Code != api.CodeCommandFailed {
				t.Fatalf("error code %s, want %s", e.Code, api.CodeCommandFailed)
			}
			return
		}
	}
}

func TestZoomBroadcastsLayout(t *testing.T) {
	s := newTestServer(t)
	a, first := attached(t, s, 80, 24)
	b := connect(t, s, 80, 24)
	b.Send(&api.Attach{Session: "work"})
	nextLayout(t, b, func(*api.Layout) bool { return true })
	a.Send(split(first, api.DirDown))
	nextLayout(t, a, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	a.Send(&api.Command{Op: api.OpZoom, Target: first})
	for _, c := range []*apitest.Client{a, b} {
		l := nextLayout(t, c, func(l *api.Layout) bool { return l.Windows[0].Zoomed })
		if p := shownPanes(l); len(p) != 1 || p[0].ID != first || p[0].Rect.H != 23 {
			t.Fatalf("zoomed layout panes %+v, want pane %d filling 80x23", p, first)
		}
	}
}

func TestCloseLastPaneClosesWindow(t *testing.T) {
	s := newTestServer(t)
	c, first := attached(t, s, 80, 24)
	c.Send(&api.Command{Op: api.OpNewWindow})
	nextLayout(t, c, func(l *api.Layout) bool { return len(l.Windows) == 2 })
	c.Send(&api.Command{Op: api.OpSelectWindow, Target: 0})
	nextLayout(t, c, func(l *api.Layout) bool { return l.Active == 0 })
	c.Send(&api.Command{Op: api.OpClose, Target: first})
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(l.Windows) == 1 })
	if l.Windows[0].Index != 1 || l.Active != 1 {
		t.Fatalf("after closing window 0's last pane: %+v, want only window 1, shown", l)
	}
}

func TestClosingEveryPaneEndsSession(t *testing.T) {
	s := newTestServer(t)
	c, first := attached(t, s, 80, 24)
	c.Send(&api.Command{Op: api.OpClose, Target: first})
	eventually(t, "the session to end", func() bool { return len(s.Sessions()) == 0 })
}

func TestResizeSwapFocusEven(t *testing.T) {
	s := newTestServer(t)
	c, first := attached(t, s, 81, 24)
	c.Send(split(first, api.DirRight))
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	second := shownPanes(l)[1].ID

	c.Send(&api.Command{Op: api.OpResize, Target: first, Args: map[string]string{api.ArgDir: api.DirRight, api.ArgCells: "10"}})
	nextLayout(t, c, func(l *api.Layout) bool { return shownPanes(l)[0].Rect.W == 50 })

	c.Send(&api.Command{Op: api.OpSwap, Target: first, Args: map[string]string{api.ArgDir: api.DirRight}})
	nextLayout(t, c, func(l *api.Layout) bool { return shownPanes(l)[0].ID == second })

	c.Send(&api.Command{Op: api.OpFocus, Target: second, Args: map[string]string{api.ArgDir: api.DirRight}})
	l = nextLayout(t, c, func(l *api.Layout) bool { return shownPanes(l)[1].Focused })
	if shownPanes(l)[1].ID != first {
		t.Fatalf("focus right from pane %d landed on %d, want %d", second, shownPanes(l)[1].ID, first)
	}

	c.Send(&api.Command{Op: api.OpEven, Target: first})
	nextLayout(t, c, func(l *api.Layout) bool { return shownPanes(l)[0].Rect.W == 40 })
}

func TestBadCommandAnswered(t *testing.T) {
	s := newTestServer(t)
	c, _ := attached(t, s, 80, 24)
	c.Send(&api.Command{Op: "explode", Target: 1})
	for {
		m, err := c.Next(5 * time.Second)
		if err != nil {
			t.Fatalf("no answer to a bad command: %v", err)
		}
		if e, ok := m.(*api.Error); ok {
			if e.Code != api.CodeBadCommand {
				t.Fatalf("error code %s, want %s", e.Code, api.CodeBadCommand)
			}
			return
		}
	}
}

func TestAttachSendsProfiles(t *testing.T) {
	s := newTestServer(t)
	c := connect(t, s, 80, 24)
	c.Send(&api.Attach{Session: "work", Create: true})
	for {
		m, err := c.Next(5 * time.Second)
		if err != nil {
			t.Fatalf("no ProfileList after Attach: %v", err)
		}
		if pl, ok := m.(*api.ProfileList); ok {
			if len(pl.Profiles) != 1 || pl.Default != pl.Profiles[0] {
				t.Fatalf("profiles %+v, want the test shell as the only default", pl)
			}
			return
		}
	}
}

// TestResizeTowardWindowBorder: with no edge on that side, resize moves the
// pane's other edge, as tmux does, so Ctrl+Left shrinks a left pane.
func TestResizeTowardWindowBorder(t *testing.T) {
	s := newTestServer(t)
	c, first := attached(t, s, 81, 24)
	c.Send(split(first, api.DirRight))
	nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	c.Send(&api.Command{Op: api.OpResize, Target: first, Args: map[string]string{api.ArgDir: api.DirLeft, api.ArgCells: "5"}})
	nextLayout(t, c, func(l *api.Layout) bool { return shownPanes(l)[0].Rect.W == 35 })
}
