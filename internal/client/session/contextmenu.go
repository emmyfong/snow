package session

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/command"
	"github.com/emmyfong/snow/internal/client/menu"
	"github.com/emmyfong/snow/internal/client/ui"
)

// openMenu shows a scrollable pane menu at cell x, y in the window area.
func (m *Model) openMenu(x, y int) {
	items := m.paneMenuItems()
	mm := menu.NewPane(items, x, y)
	if box, _, _ := mm.View(m.theme, m.cols, m.windowRows()); box != "" {
		m.menu = mm
	}
}

func (m *Model) paneMenuItems() []menu.Item {
	var items []menu.Item
	if pane := m.focused(); pane != nil {
		if git, ok := m.own[pane.ID].(gitPane); ok {
			for _, a := range git.MenuActions() {
				items = append(items, menu.Item{Label: a.Label, Shortcut: a.Shortcut, Action: "git-key:" + a.Key.String()})
			}
		}
	}
	return append(items, menu.PaneItems(m.keys)...)
}

func (m *Model) openWindowMenu() {
	_, spans := ui.WindowBarRow(m.theme, m.windowBarOptions())
	x := max(m.cols-1, 0)
	for _, sp := range spans {
		if sp.Kind == ui.TopBarMenu {
			x = sp.X
			break
		}
	}
	focused := m.focused()
	connect := focused != nil && m.canLogin(focused.ID)
	m.showMenu(menu.LayoutItems(m.keys, connect), x, 0)
}

func (m *Model) showMenu(items []menu.Item, x, y int) {
	mm := menu.New(items, x, y)
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
	if wheel, ok := msg.(tea.MouseWheelMsg); ok {
		if wheel.Button == tea.MouseWheelUp {
			m.menu.Scroll(wheelRows)
		} else if wheel.Button == tea.MouseWheelDown {
			m.menu.Scroll(-wheelRows)
		}
		return nil
	}
	click, ok := msg.(tea.MouseClickMsg)
	if !ok {
		return nil
	}
	if click.Button != tea.MouseLeft {
		m.menu = nil
		return nil
	}
	return m.menuResult(m.menu.Click(m.theme, m.cols, m.windowRows(), click.X, click.Y-m.areaTop()))
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
	if strings.HasPrefix(action, "/") {
		name, rest := command.Split(strings.TrimPrefix(action, "/"))
		return commandRuns[name](m, rest)
	}
	if name, ok := strings.CutPrefix(action, "git-key:"); ok {
		if p := m.focused(); p != nil {
			if git, ok := m.own[p.ID].(gitPane); ok {
				if k, ok := git.MenuKey(name); ok {
					return m.ownKey(k)
				}
			}
		}
		return nil
	}
	if action == "connect" {
		if p := m.focused(); p != nil {
			return m.openLoginPicker(p.ID)
		}
		return nil
	}
	return m.run(action, tea.Key{})
}

// overlayMenu draws the open menu over content, the window area of rows rows.
func (m *Model) overlayMenu(content string, rows int) string {
	if m.menu == nil {
		return content
	}
	return m.overlayPopup(content, m.menu, rows)
}
