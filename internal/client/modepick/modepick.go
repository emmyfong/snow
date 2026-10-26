// Package modepick is the approval mode picker, opened by /mode with no
// argument: both modes, what each one does, and the current one marked.
package modepick

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/listpick"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// modes are the choices, in the order the picker lists them.
var modes = []struct{ name, desc string }{
	{api.ChatModeAsk, "Ask before a tool changes files or runs commands"},
	{api.ChatModeAuto, "Run tools in the conversation folder without asking"},
}

var hints = []ui.Hint{{Key: "Enter", Desc: "choose"}, {Key: "Esc", Desc: "close"}}

// Model is the open mode picker.
type Model struct {
	list *listpick.Model
}

// New opens the picker with current marked and selected.
func New(current string) *Model {
	l := listpick.New("Approval mode", hints)
	rows := make([][]string, len(modes))
	sel := 0
	for i, m := range modes {
		mark := ""
		if m.name == current {
			mark, sel = "(current)", i
		}
		rows[i] = []string{m.name, mark, m.desc}
	}
	l.SetRows(rows)
	l.Select(sel)
	return &Model{list: l}
}

// Key handles one key press.
func (m *Model) Key(k tea.Key) picker.Result { return m.list.Key(k) }

// Click handles a left click at window cell x, y.
func (m *Model) Click(th theme.Theme, cols, rows, x, y int) picker.Result {
	return m.list.Click(th, cols, rows, x, y)
}

// View draws the picker centered on a window of cols by rows cells.
func (m *Model) View(th theme.Theme, cols, rows int) (box string, x, y int) {
	return m.list.View(th, cols, rows)
}

// Choice returns the selected mode, "ask" or "auto".
func (m *Model) Choice() string {
	if i := m.list.Selected(); i >= 0 {
		return modes[i].name
	}
	return ""
}
