package themegroup

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/command"
	"github.com/emmyfong/snow/internal/client/settings"
	"github.com/emmyfong/snow/internal/client/theme"
)

func TestThemeGroupPreviewsAndRestores(t *testing.T) {
	g := New(theme.Default())
	if got, ok := g.Update(tea.KeyPressMsg{Code: tea.KeyDown})().(command.PreviewThemeMsg); !ok || got.Theme.Name != "Campbell" {
		t.Fatalf("Down returned %#v, want Campbell preview", got)
	}
	if got, ok := g.Leave()().(command.PreviewThemeMsg); !ok || got.Theme.Name != "Frost" {
		t.Fatalf("Leave returned %#v, want Frost preview", got)
	}
}

func TestThemeGroupKeepSaves(t *testing.T) {
	g := New(theme.Default())
	cmd := g.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter returned no command")
	}
	if got, ok := cmd().(command.ThemeMsg); !ok || got.Name != "Frost" {
		t.Fatalf("Enter returned %#v, want Frost ThemeMsg", got)
	}
	if cmd := g.Leave(); cmd != nil {
		t.Fatalf("Leave after keep returned %T, want no restore", cmd())
	}
}

func TestThemeGroupEnterThenEscKeepsSaved(t *testing.T) {
	g := New(theme.Default())
	keep := g.Update(tea.KeyPressMsg{Code: tea.KeyEnter})()
	if got, ok := keep.(command.ThemeMsg); !ok || got.Name != "Frost" {
		t.Fatalf("Enter returned %#v, want Frost ThemeMsg", keep)
	}
	if got := g.Leave(); got != nil {
		t.Fatalf("Esc after Enter returned %T, want no preview revert", got())
	}
}

func TestThemeGroupClickPreviewsThenKeeps(t *testing.T) {
	g := New(theme.Default())
	g.SetSize(20, 13)
	click := tea.MouseClickMsg{X: 0, Y: 1, Button: tea.MouseLeft}
	if got, ok := g.Update(click)().(command.PreviewThemeMsg); !ok || got.Theme.Name != "Campbell" {
		t.Fatalf("click on another row returned %#v, want Campbell preview", got)
	}
	if got, ok := g.Update(click)().(command.ThemeMsg); !ok || got.Name != "Campbell" {
		t.Fatalf("click on selected row returned %#v, want Campbell ThemeMsg", got)
	}
}

func TestThemeGroupClickThroughSettings(t *testing.T) {
	g := New(theme.Default())
	m := settings.New([]settings.Group{g})
	m.SetSize(80, 24)
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	for _, click := range []tea.MouseClickMsg{
		{X: 6, Y: 3, Button: tea.MouseLeft},  // group list
		{X: 29, Y: 3, Button: tea.MouseLeft}, // divider
		{X: 75, Y: 3, Button: tea.MouseLeft}, // panel border
	} {
		if cmd := m.Update(click); cmd != nil {
			t.Fatalf("click outside the detail area returned %T", cmd())
		}
	}
	click := tea.MouseClickMsg{X: 31, Y: 3, Button: tea.MouseLeft}
	cmd := m.Update(click)
	if got, ok := cmd().(command.PreviewThemeMsg); !ok || got.Theme.Name != "Campbell" {
		t.Fatalf("click through settings returned %#v, want Campbell preview", got)
	}
	cmd = m.Update(click)
	if got, ok := cmd().(command.ThemeMsg); !ok || got.Name != "Campbell" {
		t.Fatalf("second click through settings returned %#v, want Campbell ThemeMsg", got)
	}
}

func TestThemeGroupClickMapsVisibleRows(t *testing.T) {
	g := New(theme.Default())
	g.SetSize(20, 5)
	for range 11 {
		g.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if got, ok := g.Update(tea.MouseClickMsg{X: 1, Y: 0, Button: tea.MouseLeft})().(command.PreviewThemeMsg); !ok || got.Theme.Name != "Gruvbox Dark" {
		t.Fatalf("first visible row returned %#v, want Gruvbox Dark preview", got)
	}
	for range 4 {
		g.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if got, ok := g.Update(tea.MouseClickMsg{X: 1, Y: 4, Button: tea.MouseLeft})().(command.ThemeMsg); !ok || got.Name != "Clay" {
		t.Fatalf("click on visible Clay returned %#v, want Clay ThemeMsg", got)
	}
}

func TestThemeGroupFooterClickDoesNotKeepATheme(t *testing.T) {
	g := New(theme.Default())
	m := settings.New([]settings.Group{g})
	m.SetSize(80, 12)
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	for range 7 {
		m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	cmd := m.Update(tea.MouseClickMsg{X: 6, Y: 9, Button: tea.MouseLeft})
	if cmd == nil {
		t.Fatal("footer click returned no restore command")
	}
	if got, ok := cmd().(command.PreviewThemeMsg); !ok || got.Theme.Name != "Frost" {
		t.Fatalf("footer click returned %#v, want Frost restore and no saved theme", got)
	}
}

func TestThemeGroupExternalThemeBecomesRestorePoint(t *testing.T) {
	g := New(theme.Default())
	g.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	dracula, _ := theme.ByName("Dracula")
	g.SetCommittedTheme(dracula)
	if got, ok := g.Leave()().(command.PreviewThemeMsg); !ok || got.Theme.Name != "Dracula" {
		t.Fatalf("Leave after external change returned %#v, want Dracula preview", got)
	}
}

func TestThemeGroupGolden80x24(t *testing.T) {
	themeGroupGolden(t, 80, 24)
}

func TestThemeGroupGolden120x40(t *testing.T) {
	themeGroupGolden(t, 120, 40)
}

func themeGroupGolden(t *testing.T, cols, rows int) {
	t.Helper()
	m := settings.New([]settings.Group{New(theme.Default())})
	m.SetSize(cols, rows)
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	golden.RequireEqual(t, m.View().Content)
}
