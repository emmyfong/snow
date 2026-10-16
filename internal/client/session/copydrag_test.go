package session

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/pkg/api"
)

// TestDragInPaneEntersCopyMode: dragging across a pane whose program did not
// ask for the mouse starts a copy-mode selection there, as in tmux.
func TestDragInPaneEntersCopyMode(t *testing.T) {
	m, _ := threePaneScreen(t)
	m.Update(click(5, 5)) // pane 1, which is not focused
	if m.copy != nil {
		t.Fatal("a press alone entered copy mode")
	}
	m.Update(motion(9, 6))
	if m.copy == nil || m.copy.Pane() != 1 {
		t.Fatalf("drag in pane 1: copy mode %v, want open over pane 1", m.copy)
	}
}

func TestClickInPaneDoesNotEnterCopyMode(t *testing.T) {
	m, _ := threePaneScreen(t)
	m.Update(click(5, 5))
	m.Update(release(5, 5))
	m.Update(motion(9, 6)) // a later hover with no button held
	if m.copy != nil {
		t.Fatal("a click entered copy mode")
	}
}

// TestCopyModeGetsTheMouseFirst: while copy mode is open, a press on a border
// must not start a resize.
func TestCopyModeGetsTheMouseFirst(t *testing.T) {
	m, r := threePaneScreen(t)
	m.Update(ctrlB)
	m.Update(press('[', 0, "["))
	if m.copy == nil {
		t.Fatal("prefix [ did not enter copy mode")
	}
	r.sent = nil
	m.Update(click(40, 5))
	m.Update(motion(45, 5))
	for _, s := range r.sent {
		if c, ok := s.(*api.Command); ok && c.Op == api.OpResize {
			t.Fatalf("a border drag resized while copy mode was open: %+v", c)
		}
	}
}

// TestWheelOnAPlainPaneScrollsItsHistory: the wheel over a pane whose
// program did not ask for the mouse enters copy mode there.
func TestWheelOnAPlainPaneScrollsItsHistory(t *testing.T) {
	m, _ := threePaneScreen(t)
	m.Update(tea.MouseWheelMsg{X: 5, Y: 5, Button: tea.MouseWheelUp})
	if m.copy == nil || m.copy.Pane() != 1 {
		t.Fatalf("wheel over pane 1: copy mode %v, want open over pane 1", m.copy)
	}
}

// TestWheelFocusesThePaneItOpensCopyModeOn: keys go to copy mode, so the
// pane it opens on must be the focused one.
func TestWheelFocusesThePaneItOpensCopyModeOn(t *testing.T) {
	m, r := threePaneScreen(t)
	m.Update(tea.MouseWheelMsg{X: 5, Y: 5, Button: tea.MouseWheelUp})
	c := lastCommandOf(r, api.OpFocus)
	if c == nil || c.Target != 1 {
		t.Fatalf("sent %v, want a focus command for pane 1", r.sent)
	}
}

// TestCopyModeClosesWhenItsWindowIsHidden: another client can show another
// window; copy mode over a pane no longer on screen must close, or it takes
// every key.
func TestCopyModeClosesWhenItsWindowIsHidden(t *testing.T) {
	m, _ := threePaneScreen(t)
	m.Update(ctrlB)
	m.Update(press('[', 0, "["))
	if m.copy == nil {
		t.Fatal("prefix [ did not enter copy mode")
	}
	m.Update(&api.Layout{Session: "api", Active: 2, Windows: []api.WindowInfo{
		{Index: 1, Name: "dev", Panes: threePanes(80, 24)},
		{Index: 2, Name: "logs", Panes: []api.PaneInfo{{ID: 9, Focused: true, Rect: api.Rect{W: 80, H: 23}}}},
	}})
	if m.copy != nil {
		t.Fatal("copy mode stayed open over a pane in a hidden window")
	}
}

func lastCommandOf(r *recorder, op string) *api.Command {
	for i := len(r.sent) - 1; i >= 0; i-- {
		if c, ok := r.sent[i].(*api.Command); ok && c.Op == op {
			return c
		}
	}
	return nil
}
