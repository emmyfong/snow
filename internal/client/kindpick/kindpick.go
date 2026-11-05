// Package kindpick is the pane kind picker (/open): the kinds of pane the
// server can open, with the ones the window already has marked.
package kindpick

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/listpick"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

var hints = []ui.Hint{{Key: "Enter", Desc: "open"}, {Key: "Esc", Desc: "close"}}

// Model is the picker's state.
type Model struct {
	list  *listpick.Model
	kinds []api.PaneKindInfo
}

// New lists kinds in the server's order. A kind in open is marked, since
// choosing it focuses that pane instead of adding one.
func New(kinds []api.PaneKindInfo, open map[string]bool) *Model {
	l := listpick.New("Open pane", hints)
	rows := make([][]string, len(kinds))
	for i, k := range kinds {
		mark := ""
		if open[k.Name] {
			mark = "(in this window)"
		}
		title := k.Title
		if title == "" {
			title = k.Name
		}
		rows[i] = []string{title, k.Name, mark}
	}
	l.SetRows(rows)
	l.Select(0)
	return &Model{list: l, kinds: kinds}
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

// Choice returns the selected kind's name, or "".
func (m *Model) Choice() string {
	if i := m.list.Selected(); i >= 0 && i < len(m.kinds) {
		return m.kinds[i].Name
	}
	return ""
}
