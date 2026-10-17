package home

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
)

// ThemeMsg asks the client to use the theme called Name everywhere and to
// save it in the config file. The home page already shows it.
type ThemeMsg struct{ Name string }

// setBase makes th the page's theme, adapted to the terminal's colors once
// they are known.
func (m *Model) setBase(th theme.Theme) {
	m.base, m.theme = th, th
	if m.profile != nil {
		m.theme = th.For(*m.profile)
	}
}

// cmdTheme sets the named theme, or opens the theme list when no name is
// given.
func (m *Model) cmdTheme(name string) tea.Cmd {
	if name != "" {
		th, err := theme.Named(name)
		if err != nil {
			m.notice = strings.TrimPrefix(err.Error(), theme.ErrUnknownTheme.Error()+" ")
			return nil
		}
		m.done()
		m.setBase(th)
		return func() tea.Msg { return ThemeMsg{Name: th.Name} }
	}
	m.done()
	m.picking, m.before = true, m.base
	m.themeSel = max(slices.IndexFunc(theme.Builtin(), func(t theme.Theme) bool { return t.Name == m.base.Name }), 0)
	return nil
}

// pickerKey moves through the theme list, showing each theme as it goes;
// Enter keeps the one shown and Esc puts back the one from before.
func (m *Model) pickerKey(k tea.Key) tea.Cmd {
	themes := theme.Builtin()
	switch k.Code {
	case tea.KeyUp:
		m.pickTheme(max(m.themeSel-1, 0))
	case tea.KeyDown:
		m.pickTheme(min(m.themeSel+1, len(themes)-1))
	case tea.KeyEnter:
		return m.keepTheme()
	case tea.KeyEscape:
		m.picking = false
		m.setBase(m.before)
	}
	return nil
}

// pickTheme highlights theme i and shows it on the page.
func (m *Model) pickTheme(i int) {
	m.themeSel = i
	m.setBase(theme.Builtin()[i])
}

// keepTheme closes the list with the shown theme chosen.
func (m *Model) keepTheme() tea.Cmd {
	m.picking = false
	name := m.base.Name
	return func() tea.Msg { return ThemeMsg{Name: name} }
}

// themeList draws the label and the themes that fit, the one in use marked.
func (m *Model) themeList(p placement) []string {
	th := m.theme
	muted := lipgloss.NewStyle().Foreground(th.Muted)
	lines := []string{indent([]string{muted.Render("Themes")}, p.x)[0]}
	themes := theme.Builtin()
	items := make([]ui.ListItem, 0, p.count)
	for _, t := range themes[p.first : p.first+p.count] {
		note := ""
		if t.Name == m.before.Name {
			note = "in use"
		}
		items = append(items, ui.ListItem{Cols: []string{t.Name, note}})
	}
	list := ui.List(th, ui.ListOptions{Items: items, Selected: m.themeSel - p.first, Width: p.width})
	return append(lines, indent(strings.Split(list, "\n"), p.x)...)
}
