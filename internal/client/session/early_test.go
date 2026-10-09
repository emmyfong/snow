package session

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/pkg/api"
)

// TestInputBeforeLayoutIsSent: keys typed right after snow starts, before
// the server names the pane, reach the pane in order.
func TestInputBeforeLayoutIsSent(t *testing.T) {
	r := &recorder{}
	m := New(r.send)
	m.SetSize(80, 24)
	m.Update(press('h', 0, "h"))
	m.Update(tea.PasteMsg{Content: "ello"})
	m.Update(press('!', 0, "!"))
	if len(r.sent) != 0 {
		t.Fatalf("sent %d messages before a pane was known", len(r.sent))
	}
	m.Update(&api.Layout{Session: "work", Active: 1, Windows: []api.WindowInfo{{Index: 1, Panes: []api.PaneInfo{{ID: 7, Rect: api.Rect{W: 80, H: 24}}}}}})
	var got string
	for _, msg := range r.sent {
		in := msg.(*api.Input)
		if in.Pane != 7 {
			t.Fatalf("input for pane %d, want 7", in.Pane)
		}
		if in.Key != nil {
			got += in.Key.Text
		}
		got += in.Paste
	}
	if got != "hello!" {
		t.Fatalf("pane got %q, want %q", got, "hello!")
	}
}

func TestEarlyInputIsBounded(t *testing.T) {
	r := &recorder{}
	m := New(r.send)
	for range maxEarly + 50 {
		m.Update(press('x', 0, "x"))
	}
	m.Update(&api.Layout{Session: "work", Active: 1, Windows: []api.WindowInfo{{Index: 1, Panes: []api.PaneInfo{{ID: 7, Rect: api.Rect{W: 80, H: 24}}}}}})
	if len(r.sent) != maxEarly {
		t.Fatalf("sent %d inputs, want %d", len(r.sent), maxEarly)
	}
}
