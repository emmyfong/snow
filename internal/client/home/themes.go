package home

import (
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/emmyfong/snow/internal/client/command"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/themepick"
	"github.com/emmyfong/snow/internal/client/ui"
)

// setBase makes th the page's theme, adapted to the terminal's colors once
// they are known.
func (m *Model) setBase(th theme.Theme) {
	m.base = th
	m.setVisibleTheme(th)
}

func (m *Model) setVisibleTheme(th theme.Theme) {
	m.theme = th
	if m.profile != nil {
		m.theme = th.ForBackground(*m.profile, m.paintedBackground)
	}
}

// SetPreviewTheme changes the drawing theme without changing the restore point.
func (m *Model) SetPreviewTheme(th theme.Theme) { m.setVisibleTheme(th) }

// SetPaintedBackground adapts secondary text to the background Snow applies.
// A nil color means the terminal supplies its own background.
func (m *Model) SetPaintedBackground(bg color.Color) {
	if m.paintedBackground == bg {
		return
	}
	m.paintedBackground = bg
	th := m.base
	if m.themePick != nil {
		th = m.themePick.Shown()
	}
	m.theme = th
	if m.profile != nil {
		m.theme = th.ForBackground(*m.profile, m.paintedBackground)
	}
}

// SetTheme changes the page's theme, for example when another Snow window
// picked a new one. While the theme list is open, it becomes the theme Esc
// returns to, and the one being previewed stays on screen.
func (m *Model) SetTheme(th theme.Theme) {
	if m.themePick != nil {
		m.themePick.SetBefore(th)
		return
	}
	m.setBase(th)
}

// Theme returns the theme the page draws with.
func (m *Model) Theme() theme.Theme { return m.theme }

// CurrentTheme returns the raw theme shown by the page or its open picker.
func (m *Model) CurrentTheme() theme.Theme {
	if m.themePick != nil {
		return m.themePick.Shown()
	}
	return m.base
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
		return func() tea.Msg { return command.ThemeMsg{Name: th.Name} }
	}
	m.done()
	m.themePick = themepick.New(theme.Builtin(), m.base)
	return nil
}

// pickerKey moves through the theme list, showing each theme as it goes;
// Enter keeps the one shown and Esc puts back the one from before.
func (m *Model) pickerKey(k tea.Key) tea.Cmd {
	switch k.Code {
	case tea.KeyUp:
		m.pickTheme(m.themePick.Move(-1))
	case tea.KeyDown:
		m.pickTheme(m.themePick.Move(1))
	case tea.KeyEnter:
		return m.keepTheme()
	case tea.KeyEscape:
		m.cancelTheme()
	}
	return nil
}

// pickTheme highlights theme i and shows it on the page.
func (m *Model) pickTheme(th theme.Theme) {
	m.setBase(th)
}

// keepTheme closes the list with the shown theme chosen.
func (m *Model) keepTheme() tea.Cmd {
	name := m.themePick.Keep().Name
	m.themePick = nil
	return func() tea.Msg { return command.ThemeMsg{Name: name} }
}

func (m *Model) cancelTheme() {
	if m.themePick == nil {
		return
	}
	before := m.themePick.Restore()
	m.themePick = nil
	m.setBase(before)
}

// themeList draws the label and the themes that fit, the one in use marked.
func (m *Model) themeList(p placement) []string {
	th := m.theme
	muted := lipgloss.NewStyle().Foreground(th.Muted)
	lines := []string{indent([]string{muted.Render("Themes")}, p.x)[0]}
	items := m.themePick.Items()
	visible := items[p.list.first : p.list.first+p.list.count]
	list := ui.List(th, ui.ListOptions{Items: visible, Selected: m.themePick.Selected() - p.list.first, Width: p.width})
	return append(lines, indent(strings.Split(list, "\n"), p.x)...)
}
