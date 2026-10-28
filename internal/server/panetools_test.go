package server

import (
	"testing"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/pkg/api"
)

// sessionWith attaches a new client to a new session called name and
// returns the client and the first pane's ID, without waiting for a
// prompt.
func sessionWith(t *testing.T, s *Server, name string) (*apitest.Client, int) {
	t.Helper()
	c := connect(t, s, 81, 25)
	c.Send(&api.Attach{Session: name, Create: true})
	l := nextLayout(t, c, func(*api.Layout) bool { return true })
	return c, l.Windows[0].Panes[0].ID
}

// TestListPanesSameSession: a PanesRequest lists the terminal panes of the
// caller's session only, the caller's own included, and an unknown caller
// gets CodeNoPane.
func TestListPanesSameSession(t *testing.T) {
	s := newTestServer(t)
	a, first := sessionWith(t, s, "a")
	a.Send(split(first, api.DirRight))
	l := nextLayout(t, a, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	second := shownPanes(l)[1].ID
	_, other := sessionWith(t, s, "b")

	tool := connect(t, s, 80, 24)
	tool.Send(&api.PanesRequest{Caller: second})
	got, e := apitest.Reply[*api.Panes](tool)
	if e != nil {
		t.Fatalf("PanesRequest: %v", e)
	}
	if got.Session != "a" || got.Caller != second || len(got.Panes) != 2 {
		t.Fatalf("Panes = %+v; want session a's two panes", got)
	}
	for i, want := range []int{first, second} {
		p := got.Panes[i]
		if p.ID != want || p.WindowName == "" || !p.Shown || p.Profile != shellProfile() || p.Rect.W != 40 {
			t.Errorf("pane %d = %+v; want pane %d, shown, %s, 40 wide", i, p, want, shellProfile())
		}
	}
	if !got.Panes[1].Focused || got.Panes[0].Focused {
		t.Errorf("focus %v, %v; want the new pane focused", got.Panes[0].Focused, got.Panes[1].Focused)
	}
	for _, p := range got.Panes {
		if p.ID == other {
			t.Fatalf("listed pane %d of session b", other)
		}
	}

	tool.Send(&api.PanesRequest{Caller: 999})
	if _, e := apitest.Reply[*api.Panes](tool); e == nil || e.Code != api.CodeNoPane {
		t.Fatalf("unknown caller: %v; want %s", e, api.CodeNoPane)
	}
}

// TestListPanesMarksAgents: a pane that sent a hook counts as an agent,
// with its state.
func TestListPanesMarksAgents(t *testing.T) {
	s := newTestServer(t)
	a, first := sessionWith(t, s, "a")
	a.Send(&api.AgentState{Pane: first, State: api.AgentWorking})
	nextLayout(t, a, func(l *api.Layout) bool { return shownPanes(l)[0].AgentState == api.AgentWorking })
	tool := connect(t, s, 80, 24)
	tool.Send(&api.PanesRequest{Caller: first})
	got, e := apitest.Reply[*api.Panes](tool)
	if e != nil {
		t.Fatal(e)
	}
	if p := got.Panes[0]; !p.Agent || p.AgentState != api.AgentWorking {
		t.Fatalf("pane %+v; want an agent that works", p)
	}
}
