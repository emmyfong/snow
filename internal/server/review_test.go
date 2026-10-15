package server

import (
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/pkg/api"
)

// waitPaneUpdate reads until a PaneUpdate with rows arrives for pane id.
func waitPaneUpdate(t *testing.T, c *apitest.Client, id int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		m, err := c.Next(500 * time.Millisecond)
		if err != nil {
			continue
		}
		if u, ok := m.(*api.PaneUpdate); ok && u.Pane == id && len(u.Lines) > 0 {
			return
		}
	}
	t.Fatalf("no PaneUpdate with rows for pane %d", id)
}

// TestUnzoomResendsHiddenPanes: the client forgets panes a zoomed layout
// hides, so unzooming must send their screens again.
func TestUnzoomResendsHiddenPanes(t *testing.T) {
	s := newTestServer(t)
	c, first := attached(t, s, 80, 24)
	c.Send(split(first, api.DirRight))
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	second := shownPanes(l)[1].ID
	waitPaneUpdate(t, c, second)
	c.Send(&api.Command{Op: api.OpZoom, Target: first})
	nextLayout(t, c, func(l *api.Layout) bool { return l.Windows[0].Zoomed })
	c.Drain(300 * time.Millisecond)
	c.Send(&api.Command{Op: api.OpZoom, Target: first})
	nextLayout(t, c, func(l *api.Layout) bool { return !l.Windows[0].Zoomed })
	waitPaneUpdate(t, c, second)
}

// TestFocusLeavesZoom: moving the focus out of a zoomed pane unzooms, as
// tmux does, so the focused pane is always on screen.
func TestFocusLeavesZoom(t *testing.T) {
	s := newTestServer(t)
	c, first := attached(t, s, 80, 24)
	c.Send(split(first, api.DirRight))
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	second := shownPanes(l)[1].ID
	c.Send(&api.Command{Op: api.OpZoom, Target: second})
	nextLayout(t, c, func(l *api.Layout) bool { return l.Windows[0].Zoomed })
	c.Send(&api.Command{Op: api.OpFocus, Target: second, Args: map[string]string{api.ArgDir: api.DirLeft}})
	l = nextLayout(t, c, func(l *api.Layout) bool { return !l.Windows[0].Zoomed })
	for _, p := range shownPanes(l) {
		if p.Focused != (p.ID == first) {
			t.Fatalf("panes %+v, want pane %d focused and on screen", shownPanes(l), first)
		}
	}
}

// TestFailedSplitKeepsZoom: a split whose program cannot start leaves the
// window exactly as it was, zoom included.
func TestFailedSplitKeepsZoom(t *testing.T) {
	opts := testOptions()
	good := opts.Spec
	opts.Spec = func(profile, dir string) (term.Spec, string, error) {
		if profile == "broken" {
			return term.Spec{Path: "/no/such/program-snow-test"}, "broken", nil
		}
		return good(profile, dir)
	}
	s := New(opts)
	t.Cleanup(s.Close)
	c, first := attached(t, s, 80, 24)
	c.Send(split(first, api.DirRight))
	nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	c.Send(&api.Command{Op: api.OpZoom, Target: first})
	nextLayout(t, c, func(l *api.Layout) bool { return l.Windows[0].Zoomed })
	c.Send(&api.Command{Op: api.OpSplit, Target: first, Args: map[string]string{api.ArgDir: api.DirDown, api.ArgProfile: "broken"}})
	for {
		m, err := c.Next(5 * time.Second)
		if err != nil {
			t.Fatalf("no answer to a split that cannot start: %v", err)
		}
		if _, ok := m.(*api.Error); ok {
			break
		}
	}
	var zoomed bool
	s.call(func(st *state) { zoomed = st.sessions["work"].shown().tree.Zoomed != 0 })
	if !zoomed {
		t.Fatal("the failed split turned zoom off")
	}
}
