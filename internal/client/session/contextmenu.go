package session

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/menu"
)

// openMenu shows the pane menu at screen cell x, y. It does not open when
// the screen is too small to show every item; the keys still work.
func (m *Model) openMenu(x, y int) {
	mm := menu.New(menu.PaneItems(m.keys), x, y)
	if box, _, _ := mm.View(m.theme, m.cols, m.windowRows()); box == "" {
		return
	}
	m.menu = mm
}

// menuKey handles a key while the menu is open.
func (m *Model) menuKey(k tea.Key) tea.Cmd {
	return m.menuResult(m.menu.Key(k))
}

// menuMouse handles a mouse event while the menu is open. A left click
// chooses or dismisses; a right click dismisses; the rest is ignored.
func (m *Model) menuMouse(msg tea.MouseMsg) tea.Cmd {
	click, ok := msg.(tea.MouseClickMsg)
	if !ok {
		return nil
	}
	if click.Button != tea.MouseLeft {
		m.menu = nil
		return nil
	}
	return m.menuResult(m.menu.Click(m.theme, m.cols, m.windowRows(), click.X, click.Y))
}

func (m *Model) menuResult(r menu.Result) tea.Cmd {
	switch r {
	case menu.Chosen:
		action := m.menu.Action()
		m.menu = nil
		return m.runFromMenu(action)
	case menu.Closed:
		m.menu = nil
	}
	return nil
}

// runFromMenu does an action chosen in the menu, through the same handlers
// as the keys. Close pane asks first, in the same popup as its key.
func (m *Model) runFromMenu(action string) tea.Cmd {
	return m.run(action, tea.Key{})
}

// overlayMenu draws the open menu over content, the window area of rows rows.
func (m *Model) overlayMenu(content string, rows int) string {
	if m.menu == nil {
		return content
	}
	return m.overlayPopup(content, m.menu, rows)
}
