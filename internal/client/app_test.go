package client

import (
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
