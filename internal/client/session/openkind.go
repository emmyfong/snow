package session

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/kindpick"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/pkg/api"
)

// openKind asks the server to open a pane of kind beside pane id, or to
// focus the pane of that kind its window has.
func (m *Model) openKind(id int, kind string) {
	m.send(&api.Command{Op: api.OpOpen, Target: id, Args: map[string]string{api.ArgKind: strings.ToLower(kind)}})
}

// openKindPicker shows the kinds the server can open, for pane id.
func (m *Model) openKindPicker(id int) tea.Cmd {
	if len(m.kinds) == 0 {
		return m.ShowError("This server lists no pane kinds. Update Snow.")
	}
	pick := kindpick.New(m.kinds, m.kindsInWindow(id))
	return m.showOverlay(&overlay{
		pane:  id,
		popup: pick,
		key:   pick.Key,
		click: pick.Click,
		done: func(r picker.Result) tea.Cmd {
			if kind := pick.Choice(); kind != "" && r == picker.Accepted {
				m.openKind(id, kind)
			}
			return nil
		},
	}, "the pane picker")
}

// kindsInWindow returns the kinds of the panes in the window that holds
// pane id.
func (m *Model) kindsInWindow(id int) map[string]bool {
	out := map[string]bool{}
	if m.layout == nil {
		return out
	}
	for _, w := range m.layout.Windows {
		for _, p := range w.Panes {
			if p.ID != id {
				continue
			}
			for _, q := range w.Panes {
				kind := q.Kind
				if kind == "" {
					kind = api.PaneTerminal
				}
				out[kind] = true
			}
			return out
		}
	}
	return out
}
