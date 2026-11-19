// Package themepick holds the shared state for lists that preview themes.
package themepick

import (
	"slices"

	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
)

// Model holds the themes, selected row, and theme that was in use when the
// picker opened.
type Model struct {
	themes []theme.Theme
	sel    int
	before theme.Theme
}

// New returns a picker selected on current when it is present in themes.
func New(themes []theme.Theme, current theme.Theme) *Model {
	copyOfThemes := slices.Clone(themes)
	return &Model{themes: copyOfThemes, sel: max(slices.IndexFunc(copyOfThemes, func(th theme.Theme) bool { return th.Name == current.Name }), 0), before: current}
}

// Move selects the row delta positions away, clamped to the list, and returns
// the theme to preview.
func (m *Model) Move(delta int) theme.Theme {
	if len(m.themes) > 0 {
		m.sel = min(max(m.sel+delta, 0), len(m.themes)-1)
	}
	return m.Shown()
}

// Shown returns the selected theme, or the original theme when the list is
// empty.
func (m *Model) Shown() theme.Theme {
	if len(m.themes) == 0 {
		return m.before
	}
	return m.themes[m.sel]
}

// Keep returns the selected theme.
func (m *Model) Keep() theme.Theme { return m.Shown() }

// Restore returns the theme that was in use when the picker opened.
func (m *Model) Restore() theme.Theme { return m.before }

// SetBefore changes the theme that Restore returns without changing selection.
func (m *Model) SetBefore(th theme.Theme) { m.before = th }

// Items returns the list rows and marks the original theme as in use.
func (m *Model) Items() []ui.ListItem {
	items := make([]ui.ListItem, len(m.themes))
	for i, th := range m.themes {
		note := ""
		if th.Name == m.before.Name {
			note = "in use"
		}
		items[i] = ui.ListItem{Cols: []string{th.Name, note}}
	}
	return items
}

// Selected returns the selected row index.
func (m *Model) Selected() int { return m.sel }
