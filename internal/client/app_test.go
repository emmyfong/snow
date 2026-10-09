package client

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/emmyfong/snow/internal/client/session"
	"github.com/emmyfong/snow/pkg/api"
)

func TestWindowTitleNamesTheSession(t *testing.T) {
	send := func(api.Message) {}
	a := &app{screen: session.New(send), send: send}
	if got := a.View().WindowTitle; got != "" {
		t.Fatalf("title before attaching = %q, want none", got)
	}
	a.Update(&api.Layout{Session: "work", Active: 1, Windows: []api.WindowInfo{{Index: 1, Panes: []api.PaneInfo{{ID: 1, Rect: api.Rect{W: 80, H: 24}}}}}})
	if got := a.View().WindowTitle; got != "snow: work" {
		t.Fatalf("title = %q, want %q", got, "snow: work")
	}
}

func TestHugeTerminalSendsAcceptedSize(t *testing.T) {
	var sent []api.Message
	send := func(m api.Message) { sent = append(sent, m) }
	a := &app{screen: session.New(send), send: send}
	a.Update(tea.WindowSizeMsg{Width: 5000, Height: 2000})
	r, ok := sent[len(sent)-1].(*api.Resize)
	if !ok {
		t.Fatalf("sent %T, want *api.Resize", sent[len(sent)-1])
	}
	if err := r.Validate(); err != nil {
		t.Fatalf("client sent a size the server rejects: %v", err)
	}
}

// TestUnknownSizeNotSent: without a terminal, Bubble Tea reports 0x0. That
// must not shrink a shared session to one cell.
func TestUnknownSizeNotSent(t *testing.T) {
	var sent []api.Message
	send := func(m api.Message) { sent = append(sent, m) }
	a := &app{screen: session.New(send), send: send}
	a.Update(tea.WindowSizeMsg{Width: 0, Height: 0})
	if len(sent) != 0 {
		t.Fatalf("sent %v for an unknown size, want nothing", sent)
	}
}

// TestDetachBeforeLayoutWaitsForName: a detach right after plain snow starts
// still reports which session it left.
func TestDetachBeforeLayoutWaitsForName(t *testing.T) {
	send := func(api.Message) {}
	a := &app{screen: session.New(send), send: send}
	if _, cmd := a.Update(session.DetachMsg{}); cmd != nil {
		t.Fatal("quit before the session had a name")
	}
	_, cmd := a.Update(&api.Layout{Session: "3", Active: 1, Windows: []api.WindowInfo{{Index: 1, Panes: []api.PaneInfo{{ID: 1, Rect: api.Rect{W: 80, H: 24}}}}}})
	if cmd == nil {
		t.Fatal("did not quit after the Layout")
	}
	if !a.result.Detached || a.result.Session != "3" {
		t.Fatalf("result %+v, want detached from 3", a.result)
	}
}

func TestEndAndLostConnectionDiffer(t *testing.T) {
	newApp := func() *app {
		send := func(api.Message) {}
		return &app{screen: session.New(send), send: send}
	}
	ended := newApp()
	ended.Update(&api.Error{Code: api.CodeSessionEnded, Message: "session work ended"})
	ended.Update(connClosedMsg{})
	if !ended.result.Ended || ended.result.Err != nil {
		t.Fatalf("after session-ended: %+v, want Ended and no error", ended.result)
	}

	lost := newApp()
	lost.Update(connClosedMsg{})
	if lost.result.Ended || !errors.Is(lost.result.Err, ErrConnectionLost) {
		t.Fatalf("after a bare close: %+v, want ErrConnectionLost", lost.result)
	}
}
