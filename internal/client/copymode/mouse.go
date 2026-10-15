package copymode

import (
	tea "charm.land/bubbletea/v2"
)

// wheelLines is how far one notch of the mouse wheel scrolls.
const wheelLines = 3

// Mouse handles a mouse event at cell x, y of the pane's area: the wheel
// scrolls, a drag with the left button selects, and releasing the button
// copies the selection and leaves copy mode. It only sees events once the
// client turns on mouse reporting, which is not copy mode's to decide.
func (m *Model) Mouse(msg tea.MouseMsg, x, y int) (Result, tea.Cmd) {
	if !m.ready {
		return Open, nil
	}
	inside := x >= 0 && x < m.cols && y >= 0 && y < m.rows
	switch ev := msg.(type) {
	case tea.MouseWheelMsg:
		switch ev.Button {
		case tea.MouseWheelUp:
			m.scroll(-wheelLines)
		case tea.MouseWheelDown:
			m.scroll(wheelLines)
		}
	case tea.MouseClickMsg:
		if ev.Button == tea.MouseLeft && inside {
			m.cur = m.at(x, y)
			m.want = m.cur.col
			m.selecting, m.dragging = false, true
			m.anchor = m.cur
		}
	case tea.MouseMotionMsg:
		if m.dragging {
			m.cur = m.at(x, y)
			m.want = m.cur.col
			m.reveal()
			m.selecting = m.cur != m.anchor
		}
	case tea.MouseReleaseMsg:
		if m.dragging {
			m.dragging = false
			m.cur = m.at(x, y)
			m.reveal()
			m.fill()
			if m.cur != m.anchor {
				m.selecting = true
				return m.copySelection()
			}
		}
	}
	m.fill()
	return Open, nil
}

// at returns the history cell under cell x, y of the view. Outside the
// view it is the nearest line past the edge, so a drag above or below the
// pane scrolls as it goes.
func (m *Model) at(x, y int) pos {
	line := min(max(m.top+y, m.top-1), m.top+m.rows)
	line = min(max(line, 0), max(m.total-1, 0))
	col := min(max(x, 0), max(m.width(line)-1, 0))
	return pos{line: line, col: col}
}

// scroll moves the view by n lines and keeps the cursor in it.
func (m *Model) scroll(n int) {
	m.top = min(max(m.top+n, 0), m.bottom())
	m.cur.line = min(max(m.cur.line, m.top), min(m.top+m.rows-1, max(m.total-1, 0)))
	m.settle()
}
