package server

import (
	"errors"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/pkg/api"
)

// hook sends one agent state from its own connection, as snow hook does.
func hook(t *testing.T, s *Server, pane int, state string, at time.Time) {
	t.Helper()
	h := connect(t, s, 80, 24)
	h.Send(&api.AgentState{Pane: pane, State: state, At: at})
	h.Close()
}

// paneAgent returns the agent state of pane id in l, and whether l has it.
func paneAgent(l *api.Layout, id int) (string, bool) {
	for _, w := range l.Windows {
		for _, p := range w.Panes {
			if p.ID == id {
				return p.AgentState, true
			}
		}
	}
	return "", false
}

func TestAgentStateInLayout(t *testing.T) {
	s := newTestServer(t)
	c, first := attachNoPrompt(t, s)
	t0 := time.Now()
	for i, state := range []string{api.AgentWorking, api.AgentNeedsYou, api.AgentWorking, api.AgentIdle} {
		hook(t, s, first, state, t0.Add(time.Duration(i)*time.Millisecond))
		l := nextLayout(t, c, func(l *api.Layout) bool { got, _ := paneAgent(l, first); return got == state })
		if l.Windows[0].AgentState != state {
			t.Fatalf("window state %q, want %q", l.Windows[0].AgentState, state)
		}
	}
	hook(t, s, first, "", t0.Add(time.Second))
	nextLayout(t, c, func(l *api.Layout) bool { got, _ := paneAgent(l, first); return got == "" })

	// A closed pane loses its state: a new pane in its place starts with
	// none.
	c.Send(split(first, api.DirRight))
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	second := shownPanes(l)[1].ID
	hook(t, s, second, api.AgentNeedsYou, t0.Add(2*time.Second))
	nextLayout(t, c, func(l *api.Layout) bool { return l.Windows[0].AgentState == api.AgentNeedsYou })
	c.Send(&api.Command{Op: api.OpClose, Target: second})
	l = nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 1 })
	if l.Windows[0].AgentState != "" {
		t.Fatalf("window state %q after its agent's pane closed, want none", l.Windows[0].AgentState)
	}
}

// TestHookIgnoresOtherPanes: a state for a pane that is gone or never was
// changes nothing, and one for another session's pane leaves this session
// alone.
func TestHookIgnoresOtherPanes(t *testing.T) {
	s := newTestServer(t)
	c, _ := attachNoPrompt(t, s)
	other := connect(t, s, 80, 24)
	other.Send(&api.Attach{Session: "other", Create: true})
	ol := nextLayout(t, other, func(*api.Layout) bool { return true })
	otherPane := ol.Windows[0].Panes[0].ID

	hook(t, s, 999, api.AgentNeedsYou, time.Now())
	hook(t, s, otherPane, api.AgentNeedsYou, time.Now())
	nextLayout(t, other, func(l *api.Layout) bool { got, _ := paneAgent(l, otherPane); return got == api.AgentNeedsYou })
	expectNoAgentLayout(t, c)
}

// expectNoAgentLayout fails when c gets a layout with an agent state soon.
func expectNoAgentLayout(t *testing.T, c *apitest.Client) {
	t.Helper()
	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		m, err := c.Next(100 * time.Millisecond)
		if err != nil && !errors.Is(err, apitest.ErrTimeout) {
			t.Fatal(err)
		}
		if l, ok := m.(*api.Layout); ok {
			for _, w := range l.Windows {
				if w.AgentState != "" {
					t.Fatalf("layout %+v has an agent state", l)
				}
			}
		}
	}
}

// TestAgentStateOlderIgnored: two hooks can race; the one that ran last
// wins even when it arrives first.
func TestAgentStateOlderIgnored(t *testing.T) {
	s := newTestServer(t)
	c, first := attachNoPrompt(t, s)
	now := time.Now()
	hook(t, s, first, api.AgentNeedsYou, now)
	nextLayout(t, c, func(l *api.Layout) bool { return l.Windows[0].AgentState == api.AgentNeedsYou })
	hook(t, s, first, api.AgentWorking, now.Add(-time.Second))
	hook(t, s, first, api.AgentIdle, now.Add(time.Second))
	l := nextLayout(t, c, func(l *api.Layout) bool { return l.Windows[0].AgentState != api.AgentNeedsYou })
	if l.Windows[0].AgentState != api.AgentIdle {
		t.Fatalf("state %q, want idle: the older working report must be ignored", l.Windows[0].AgentState)
	}
}

// TestZoomedWindowKeepsAgentState: a pane a zoom hides still sets its
// window's state, so the status bar does not lose a waiting agent.
func TestZoomedWindowKeepsAgentState(t *testing.T) {
	s := newTestServer(t)
	c, first := attachNoPrompt(t, s)
	c.Send(split(first, api.DirRight))
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	second := shownPanes(l)[1].ID
	c.Send(&api.Command{Op: api.OpZoom, Target: second})
	nextLayout(t, c, func(l *api.Layout) bool { return l.Windows[0].Zoomed })
	hook(t, s, first, api.AgentNeedsYou, time.Now())
	l = nextLayout(t, c, func(l *api.Layout) bool { return l.Windows[0].AgentState == api.AgentNeedsYou })
	if _, shown := paneAgent(l, first); shown {
		t.Fatal("the zoom should hide the agent's pane")
	}
}

// TestAgentStateFutureAt: a report stamped in the future, from a clock
// that runs ahead, does not freeze the pane's state.
func TestAgentStateFutureAt(t *testing.T) {
	s := newTestServer(t)
	c, first := attachNoPrompt(t, s)
	hook(t, s, first, api.AgentNeedsYou, time.Now().Add(time.Hour))
	nextLayout(t, c, func(l *api.Layout) bool { return l.Windows[0].AgentState == api.AgentNeedsYou })
	time.Sleep(10 * time.Millisecond)
	hook(t, s, first, api.AgentWorking, time.Now())
	nextLayout(t, c, func(l *api.Layout) bool { return l.Windows[0].AgentState == api.AgentWorking })
}

// TestWindowShowsMostUrgent: a window with two agents carries the more
// urgent state, for its status bar tab.
func TestWindowShowsMostUrgent(t *testing.T) {
	s := newTestServer(t)
	c, first := attachNoPrompt(t, s)
	c.Send(split(first, api.DirRight))
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	second := shownPanes(l)[1].ID
	now := time.Now()
	hook(t, s, first, api.AgentIdle, now)
	nextLayout(t, c, func(l *api.Layout) bool { return l.Windows[0].AgentState == api.AgentIdle })
	hook(t, s, second, api.AgentWorking, now)
	nextLayout(t, c, func(l *api.Layout) bool { return l.Windows[0].AgentState == api.AgentWorking })
	hook(t, s, first, api.AgentNeedsYou, now.Add(time.Millisecond))
	nextLayout(t, c, func(l *api.Layout) bool { return l.Windows[0].AgentState == api.AgentNeedsYou })
}
