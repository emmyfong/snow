package copymode

import tea "charm.land/bubbletea/v2"

// Key handles one key press and tells the session whether copy mode is
// still open. Keys follow tmux's vi table, with the arrow keys and page keys
// as well.
func (m *Model) Key(k tea.Key) (Result, tea.Cmd) {
	// Any key cancels a copy that waits for lines: the user moved on.
	m.copying = false
	if m.search.typing {
		m.promptKey(k)
		return Open, nil
	}
	m.notice = ""
	if k.Code == tea.KeyEscape && m.selecting {
		m.selecting = false
		return Open, nil
	}
	if k.Code == tea.KeyEscape || (k.Text == "q" && k.Mod == 0) {
		return Exited, nil
	}
	if !m.ready {
		return Open, nil
	}
	if k.Mod == tea.ModCtrl {
		switch k.Code {
		case 'b':
			m.page(-m.rows)
		case 'f':
			m.page(m.rows)
		case 'u':
			m.page(-m.rows / 2)
		case 'd':
			m.page(m.rows / 2)
		}
		m.fill()
		return Open, nil
	}
	switch {
	case k.Code == tea.KeyUp, k.Text == "k":
		m.moveLines(-1)
	case k.Code == tea.KeyDown, k.Text == "j":
		m.moveLines(1)
	case k.Code == tea.KeyLeft, k.Text == "h":
		m.moveCol(-1)
	case k.Code == tea.KeyRight, k.Text == "l":
		m.moveCol(1)
	case k.Code == tea.KeyHome, k.Text == "0":
		m.moveCol(-m.cur.col)
	case k.Code == tea.KeyEnd, k.Text == "$":
		m.moveCol(m.width(m.cur.line))
	case k.Code == tea.KeyPgUp:
		m.page(-m.rows)
	case k.Code == tea.KeyPgDown:
		m.page(m.rows)
	case k.Text == "g":
		m.home()
	case k.Text == "G":
		m.end()
	case k.Text == "v":
		m.toggleSelection()
	case k.Text == "y", k.Code == tea.KeyEnter:
		return m.copySelection()
	case k.Text == "?":
		m.startSearch(true)
	case k.Text == "/":
		m.startSearch(false)
	case k.Text == "n":
		m.repeat(false)
	case k.Text == "N":
		m.repeat(true)
	}
	m.fill()
	return Open, nil
}
