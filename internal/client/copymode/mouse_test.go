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
