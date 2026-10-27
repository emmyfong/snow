// Package menu is the right-click pane menu: a popup at the click position
// that lists the common pane actions and, beside each, the keys that run it.
// Mouse users get every action without a toolbar, and learn the shortcuts as
// they go.
package menu

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/keys"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
)

// Item is one row: a label, the shortcut that does the same by keyboard
// ("" when no key is bound), and the action it runs.
type Item struct {
	Label    string
	Shortcut string
	Action   string
}

// paneActions are the actions the pane menu offers, in order.
var paneActions = []string{"split-right", "split-down", "new-claude", "zoom", "even-out", "new-window", "rename-window", "close-pane", "close-window", "sessions", "command-prompt", "detach"}

// PaneItems returns the pane menu's items. Labels and shortcuts come from
// the key table, so a rebound key shows its new name.
func PaneItems(t *keys.Table) []Item {
	items := make([]Item, 0, len(paneActions))
	for _, name := range paneActions {
		a, ok := keys.ActionByName(name)
		if !ok {
			continue
		}
		items = append(items, Item{Label: a.Desc, Shortcut: t.Shortcut(name), Action: name})
	}
	return items
}

// Result is what a key press or a click did to the menu.
type Result int

// Results.
const (
	Open   Result = iota // the menu stays open
	Chosen               // Action holds the chosen action
	Closed               // the user dismissed the menu
)

// Model is one open menu.
type Model struct {
	items    []Item
	selected int
	x, y     int // where the user clicked
}

// New opens a menu for items at screen cell x, y.
func New(items []Item, x, y int) *Model { return &Model{items: items, x: x, y: y} }

// Selected returns the index of the highlighted item.
func (m *Model) Selected() int { return m.selected }

// Action returns the chosen item's action. It is valid after a Result of
// Chosen.
func (m *Model) Action() string {
	if m.selected < 0 || m.selected >= len(m.items) {
		return ""
	}
	return m.items[m.selected].Action
}

// Key handles one key press: arrows or j and k move, Enter chooses, Escape
// closes.
func (m *Model) Key(k tea.Key) Result {
	n := len(m.items)
	switch {
	case k.Code == tea.KeyEscape:
		return Closed
	case k.Code == tea.KeyEnter:
		if n == 0 {
			return Closed
		}
		return Chosen
	case n == 0:
	case k.Code == tea.KeyUp || k.Text == "k":
		m.selected = (m.selected - 1 + n) % n
	case k.Code == tea.KeyDown || k.Text == "j":
		m.selected = (m.selected + 1) % n
	}
	return Open
}

// Click handles a left click at screen cell x, y on a screen of cols by rows
// cells. A click on an item chooses it, a click on the frame does nothing,
// and a click anywhere else closes the menu.
func (m *Model) Click(th theme.Theme, cols, rows, x, y int) Result {
	box, bx, by := m.View(th, cols, rows)
	if box == "" {
		return Closed
	}
	w, h := ui.Size(box)
	if x < bx || x >= bx+w || y < by || y >= by+h {
		return Closed
	}
	if i := y - by - 1; x > bx && x < bx+w-1 && i >= 0 && i < len(m.items) {
		m.selected = i
		return Chosen
	}
	return Open
}

// View draws the menu for a screen of cols by rows cells and returns the
// box with the cell where its top-left corner goes: at the click, moved left
// and up as far as needed to stay on screen. It returns "" when nothing
// fits.
func (m *Model) View(th theme.Theme, cols, rows int) (box string, x, y int) {
	if len(m.items) == 0 {
		return "", 0, 0
	}
	labelW, hintW := 0, 0
	items := make([]string, len(m.items))
	hints := make([]string, len(m.items))
	for i, it := range m.items {
		items[i], hints[i] = it.Label, it.Shortcut
		labelW, hintW = max(labelW, ansi.StringWidth(it.Label)), max(hintW, ansi.StringWidth(it.Shortcut))
	}
	// Frame, pointer, label, a gap, the hint, and a margin.
	width := 2 + ansi.StringWidth(th.Glyphs.Pointer) + labelW + 1
	if hintW > 0 {
		width += 2 + hintW
	}
	width = min(width, cols)
	height := len(m.items) + 2
	if width < 4 || height > rows {
		// A menu that cannot show every item is no use; the keys still work.
		return "", 0, 0
	}
	box = ui.Popup(th, ui.PopupOptions{Items: items, Hints: hints, Selected: m.selected, Width: width})
	x = max(min(m.x, cols-width), 0)
	y = max(min(m.y, rows-height), 0)
	return strings.TrimRight(box, "\n"), x, y
}
