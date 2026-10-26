package session

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/copymode"
	"github.com/emmyfong/snow/pkg/api"
)

// enterCopy opens copy mode over the focused pane. It does nothing before a
// pane is known, or when copy mode is already open.
func (m *Model) enterCopy() {
	if p := m.focused(); p != nil {
		m.enterCopyOn(p.ID)
	}
}

// enterCopyOn opens copy mode over pane id of the shown window and reports
// whether it is open there.
func (m *Model) enterCopyOn(id int) bool {
	if m.copy != nil {
		return m.copy.Pane() == id
	}
	p := m.paneByID(id)
	if p == nil || m.own[id] != nil {
		// A pane the client draws has no terminal history; it scrolls by
		// itself.
		return false
	}
	o := copymode.Options{Pane: p.ID, Cols: p.Rect.W, Rows: p.Rect.H, Send: m.send, Copy: m.setClip}
	if st := m.panes[p.ID]; st != nil {
		o.Screen = append([]string(nil), st.lines...)
		o.CursorX, o.CursorY = st.cursor.X, st.cursor.Y
	}
	m.copy = copymode.New(o)
	return true
}

// copyKey sends a key to copy mode and closes it when it exits.
func (m *Model) copyKey(k tea.Key) tea.Cmd {
	res, cmd := m.copy.Key(k)
	return m.afterCopy(res, cmd)
}

// copyReceive gives copy mode a message from the server.
func (m *Model) copyReceive(msg api.Message) tea.Cmd {
	if m.copy == nil {
		return nil
	}
	res, cmd := m.copy.Receive(msg)
	return m.afterCopy(res, cmd)
}

// copyMouse gives copy mode a mouse event, with the cell made relative to
// the pane's area.
func (m *Model) copyMouse(msg tea.MouseMsg) tea.Cmd {
	p := m.paneByID(m.copy.Pane())
	if p == nil {
		return nil
	}
	mouse := msg.Mouse()
	res, cmd := m.copy.Mouse(msg, mouse.X-p.Rect.X, mouse.Y-p.Rect.Y)
	return m.afterCopy(res, cmd)
}

// afterCopy closes copy mode when it has exited. What copy mode last said,
// such as how much it copied, stays in the status bar for a moment.
func (m *Model) afterCopy(res copymode.Result, cmd tea.Cmd) tea.Cmd {
	if res != copymode.Exited {
		return cmd
	}
	note := m.copy.Status()
	m.copy = nil
	if note == "" {
		return cmd
	}
	return tea.Batch(cmd, m.showNotice(note))
}

// syncCopy keeps copy mode in step with the layout: it follows the pane's
// size, and closes when the pane is gone or no longer on screen, for example
// when another client shows another window. Open over a hidden pane, it
// would take every key.
func (m *Model) syncCopy() {
	if m.copy == nil {
		return
	}
	if p := m.paneByID(m.copy.Pane()); p != nil {
		m.copy.Resize(p.Rect.W, p.Rect.H)
		return
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
