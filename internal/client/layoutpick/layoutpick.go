// Package layoutpick is the layout preset picker (/layout): the presets of
// api.LayoutPresets, each with what it holds.
package layoutpick

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/listpick"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

var hints = []ui.Hint{{Key: "Enter", Desc: "arrange"}, {Key: "Esc", Desc: "close"}}

// Model is the picker's state.
type Model struct {
	list *listpick.Model
}

// New lists api.LayoutPresets.
func New() *Model {
	l := listpick.New("Layout", hints)
	rows := make([][]string, len(api.LayoutPresets))
	for i, p := range api.LayoutPresets {
		rows[i] = []string{p.Name, p.Desc}
	}
	l.SetRows(rows)
	l.Select(0)
	return &Model{list: l}
}

// Key handles a key press.
func (m *Model) Key(k tea.Key) picker.Result { return m.list.Key(k) }

// Click handles a click at x, y on a screen of cols by rows cells.
func (m *Model) Click(th theme.Theme, cols, rows, x, y int) picker.Result {
	return m.list.Click(th, cols, rows, x, y)
}

// View draws the picker centered on a screen of cols by rows cells.
func (m *Model) View(th theme.Theme, cols, rows int) (box string, x, y int) {
	return m.list.View(th, cols, rows)
}

// Choice returns the selected preset's name, or "".
func (m *Model) Choice() string {
	if i := m.list.Selected(); i >= 0 && i < len(api.LayoutPresets) {
		return api.LayoutPresets[i].Name
	}
	return ""
}
