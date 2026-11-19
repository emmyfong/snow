// Package themegroup implements the theme section of the settings screen.
package themegroup

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/command"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/themepick"
	"github.com/emmyfong/snow/internal/client/ui"
)

// Group is the theme section of settings.
type Group struct {
	pick   *themepick.Model
	before theme.Theme
	width  int
	height int
}

// New returns the group with the current theme selected.
func New(current theme.Theme) *Group {
	return &Group{pick: themepick.New(theme.Builtin(), current), before: current}
}

// Title is the label shown in the settings group list.
func (*Group) Title() string { return "Theme" }

// Update handles theme selection and preview input.
func (g *Group) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyUp:
			return preview(g.pick.Move(-1))
		case tea.KeyDown:
			return preview(g.pick.Move(1))
		case tea.KeyEnter:
			return g.keep()
		}
	case tea.MouseClickMsg:
		if msg.Button != tea.MouseLeft || msg.X < 0 || msg.X >= g.width || msg.Y < 0 || msg.Y >= g.height {
			return nil
		}
		first, last := ui.ListWindow(len(g.pick.Items()), g.pick.Selected(), g.height)
		index := first + msg.Y
		if index >= last {
			return nil
		}
		if index == g.pick.Selected() {
			return g.keep()
		}
		return preview(g.pick.Move(index - g.pick.Selected()))
	}
	return nil
}

// SetSize sets the visible theme list area for mouse hit testing.
func (g *Group) SetSize(width, height int) { g.width, g.height = width, height }

// SetCommittedTheme updates the theme to restore after an external config change.
func (g *Group) SetCommittedTheme(th theme.Theme) {
	previewing := g.pick.Shown().Name != g.before.Name
	g.before = th
	g.pick.SetBefore(th)
	if !previewing {
		g.pick = themepick.New(theme.Builtin(), th)
	}
}

// ThemePreview reports the current temporary selection, if it differs from the restore point.
func (g *Group) ThemePreview() (theme.Theme, bool) {
	th := g.pick.Shown()
	return th, th.Name != g.before.Name
}

// Leave restores the theme from before the group opened or the last one kept.
func (g *Group) Leave() tea.Cmd {
	if g.pick.Shown().Name == g.before.Name {
		return nil
	}
	before := g.before
	g.pick = themepick.New(theme.Builtin(), before)
	return preview(before)
}

// View draws the built-in themes in the supplied area.
func (g *Group) View(th theme.Theme, width, height int) string {
	return ui.List(th, ui.ListOptions{Items: g.pick.Items(), Selected: g.pick.Selected(), Width: width, Height: height})
}

func (g *Group) keep() tea.Cmd {
	th := g.pick.Keep()
	g.before = th
	g.pick.SetBefore(th)
	return func() tea.Msg { return command.ThemeMsg{Name: th.Name} }
}

func preview(th theme.Theme) tea.Cmd {
	return func() tea.Msg { return command.PreviewThemeMsg{Theme: th} }
}
