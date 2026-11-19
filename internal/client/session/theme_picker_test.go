package session

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/command"
	"github.com/emmyfong/snow/internal/client/theme"
)

func openThemePicker(t *testing.T) *Model {
	t.Helper()
	m, _ := openPrompt(t, 80, 24)
	typeIn(m, "theme\r")
	if m.prompt == nil || m.prompt.mode != modeThemes {
		t.Fatal("theme command did not open the theme picker")
	}
	return m
}

func TestCommandThemesPreviewAndEsc(t *testing.T) {
	m := openThemePicker(t)
	if cmd := m.Update(press(tea.KeyDown, 0, "")); cmd != nil {
		t.Fatalf("Down returned command %v, want none", cmd)
	}
	if m.theme.Name != "Campbell" {
		t.Fatalf("theme after Down = %q, want Campbell", m.theme.Name)
	}
	if cmd := m.Update(press(tea.KeyEscape, 0, "")); cmd != nil {
		t.Fatalf("Esc returned command %v, want none", cmd)
	}
	if m.prompt != nil || m.theme.Name != "Frost" {
		t.Fatalf("after Esc prompt=%v theme=%q, want closed and Frost", m.prompt, m.theme.Name)
	}
}

func TestCommandThemesClickOutsideRestores(t *testing.T) {
	m := openThemePicker(t)
	m.Update(press(tea.KeyDown, 0, ""))
	m.Update(click(0, 0))
	if m.prompt != nil || m.theme.Name != "Frost" {
		t.Fatalf("after outside click prompt=%v theme=%q, want closed and Frost", m.prompt, m.theme.Name)
	}
}

func TestCommandThemesEnterKeeps(t *testing.T) {
	m := openThemePicker(t)
	m.Update(press(tea.KeyDown, 0, ""))
	if got, ok := themeOf(m.Update(enter)); !ok || got != (command.ThemeMsg{Name: "Campbell"}) {
		t.Fatalf("Enter gave %#v, want Campbell ThemeMsg", got)
	}
	if m.prompt != nil || m.theme.Name != "Campbell" {
		t.Fatalf("after Enter prompt=%v theme=%q, want closed and Campbell", m.prompt, m.theme.Name)
	}
}

func TestSetThemeWhileThemesOpen(t *testing.T) {
	m := openThemePicker(t)
	m.Update(press(tea.KeyDown, 0, ""))
	dracula, _ := theme.ByName("Dracula")
	m.SetTheme(dracula)
	if m.theme.Name != "Campbell" {
		t.Fatalf("external theme update replaced preview with %q", m.theme.Name)
	}
	m.Update(press(tea.KeyEscape, 0, ""))
	if m.theme.Name != "Dracula" {
		t.Fatalf("Esc returned to %q, want Dracula", m.theme.Name)
	}
}
