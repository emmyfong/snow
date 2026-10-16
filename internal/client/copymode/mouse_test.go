package copymode

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/pkg/api"
)

// TestPressBeforeReadyStartsTheSelection: a drag that enters copy mode sends
// its press before the history arrives; the press must not be lost.
func TestPressBeforeReadyStartsTheSelection(t *testing.T) {
	m := New(Options{Pane: 1, Cols: 20, Rows: 4, Screen: []string{"alpha", "beta", "gamma", "delta"}, Send: func(api.Message) {}})
	m.Mouse(tea.MouseClickMsg{X: 1, Y: 1, Button: tea.MouseLeft}, 1, 1)
	m.Receive(&api.Scrollback{Pane: 1, From: 0, Total: 4, Lines: []string{"alpha", "beta", "gamma", "delta"}})
	if !m.dragging {
		t.Fatal("the early press was lost")
	}
}

// TestQuickDragBeforeReadyCopies: a drag that ends before the history
// arrives still selects and copies once it does.
func TestQuickDragBeforeReadyCopies(t *testing.T) {
	var copied []string
	lines := []string{"alpha", "beta", "gamma", "delta"}
	m := New(Options{Pane: 1, Cols: 20, Rows: 4, Screen: lines, Send: func(api.Message) {},
		Copy: func(text string) tea.Cmd { copied = append(copied, text); return nil }})
	m.Mouse(tea.MouseClickMsg{X: 1, Y: 1, Button: tea.MouseLeft}, 1, 1)
	m.Mouse(tea.MouseMotionMsg{X: 3, Y: 2, Button: tea.MouseLeft}, 3, 2)
	m.Mouse(tea.MouseMotionMsg{X: 4, Y: 2, Button: tea.MouseLeft}, 4, 2)
	m.Mouse(tea.MouseReleaseMsg{X: 4, Y: 2, Button: tea.MouseLeft}, 4, 2)
	res, _ := m.Receive(&api.Scrollback{Pane: 1, From: 0, Total: 4, Lines: lines})
	if res != Exited || len(copied) != 1 || copied[0] != "eta\ngamma" {
		t.Fatalf("result %v, copied %q; want Exited and \"eta\\ngamma\"", res, copied)
	}
	if m.dragging {
		t.Fatal("the drag is still held after its release")
	}
}
