package session

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/copymode"
)

// enterCopy opens copy mode over the focused pane. It does nothing before a
// pane is known, or when copy mode is already open.
func (m *Model) enterCopy() {
	p := m.focused()
	if p == nil || m.copy != nil {
		return
	}
	o := copymode.Options{Pane: p.ID, Cols: p.Rect.W, Rows: p.Rect.H, Send: m.send}
	if st := m.panes[p.ID]; st != nil {
		o.Screen = append([]string(nil), st.lines...)
		o.CursorX, o.CursorY = st.cursor.X, st.cursor.Y
	}
	m.copy = copymode.New(o)
}

// copyKey sends a key to copy mode and closes it when it exits.
func (m *Model) copyKey(k tea.Key) tea.Cmd {
	res, cmd := m.copy.Key(k)
	if res == copymode.Exited {
		m.copy = nil
	}
	return cmd
}

// syncCopy keeps copy mode in step with the layout: it closes when its pane
// is gone and follows the pane's size.
func (m *Model) syncCopy() {
	if m.copy == nil {
		return
	}
	for _, w := range m.layout.Windows {
		for _, p := range w.Panes {
			if p.ID == m.copy.Pane() {
				m.copy.Resize(p.Rect.W, p.Rect.H)
				return
			}
		}
	}
	m.copy = nil
}

// copyRows returns copy mode's rows for pane id, or nil when copy mode is
// not open over that pane.
func (m *Model) copyRows(id int) []string {
	if m.copy == nil || m.copy.Pane() != id {
		return nil
	}
	return strings.Split(m.copy.View(m.theme), "\n")
}

// copyCursor returns copy mode's cursor in pane id's area, if it has one.
func (m *Model) copyCursor(id int) (x, y int, ok bool) {
	if m.copy == nil || m.copy.Pane() != id {
		return 0, 0, false
	}
	return m.copy.Cursor()
}
