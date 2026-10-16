package session

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/copymode"
)

// paneDrag is a left press in a pane whose program did not ask for the
// mouse. If the pointer moves before the button is released, it becomes a
// copy-mode selection, as in tmux; a release first leaves it a plain click.
type paneDrag struct {
	pane, x, y int // the pane and the cell pressed, in pane coordinates
}

// startPaneDrag remembers a left press in pane id at cell x, y of the pane.
func (m *Model) startPaneDrag(id, x, y int) {
	m.dragFrom = &paneDrag{pane: id, x: x, y: y}
}

// dragToCopy turns a remembered press into a copy-mode selection when the
// pointer moves with the button held: copy mode opens over that pane and
// gets the press, then the motion.
func (m *Model) dragToCopy(ev tea.Mouse) tea.Cmd {
	d := m.dragFrom
	m.dragFrom = nil
	if !m.enterCopyOn(d.pane) {
		return nil
	}
	res, cmd := m.copy.Mouse(tea.MouseClickMsg{Button: tea.MouseLeft}, d.x, d.y)
	if res == copymode.Exited {
		return m.afterCopy(res, cmd)
	}
	return tea.Batch(cmd, m.copyMouse(tea.MouseMotionMsg(ev)))
}
