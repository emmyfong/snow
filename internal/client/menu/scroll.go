package menu

import "github.com/emmyfong/snow/internal/client/ui"

// NewPane opens a pane menu. When the items exceed the screen height, its
// viewport follows the selection and adds clickable scroll controls.
// New retains its existing requirement that all items fit together.
func NewPane(items []Item, x, y int) *Model {
	m := New(items, x, y)
	m.scrollable = true
	return m
}

// Scroll moves the selection of a pane menu toward earlier items for
// positive rows. Other menus ignore scrolling.
func (m *Model) Scroll(rows int) {
	if m.scrollable && len(m.items) > 0 {
		m.selected = min(max(m.selected-rows, 0), len(m.items)-1)
	}
}

// viewport supplies the same item window to rendering and mouse hits.
func (m *Model) viewport(rows int) (first, last int, scrolling bool) {
	if len(m.items)+2 <= rows {
		return 0, len(m.items), false
	}
	if !m.scrollable || rows < 5 {
		return 0, 0, false
	}
	first, last = ui.ListWindow(len(m.items), m.selected, rows-4)
	return first, last, true
}
