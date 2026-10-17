package session

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// TestColorProfileKeepsTheConfiguredTheme: the terminal reports its color
// depth after start; the screen adapts the theme from the config, not the
// default one.
func TestColorProfileKeepsTheConfiguredTheme(t *testing.T) {
	nord, _ := theme.ByName("Nord")
	m := New(func(api.Message) {}, WithTheme(nord))
	m.Update(tea.ColorProfileMsg{Profile: colorprofile.TrueColor})
	if m.theme.Name != "Nord" {
		t.Fatalf("theme %q after the color profile, want Nord", m.theme.Name)
	}
}

// TestSetThemeRedraws: a theme changed elsewhere reaches an open session
// screen, still adapted to the terminal's colors.
func TestSetThemeRedraws(t *testing.T) {
	m := New(func(api.Message) {})
	m.Update(tea.ColorProfileMsg{Profile: colorprofile.ANSI})
	dracula, _ := theme.ByName("Dracula")
	m.SetTheme(dracula)
	if m.theme.Name != "Dracula" || m.theme != dracula.For(colorprofile.ANSI) {
		t.Fatal("SetTheme did not apply Dracula for the terminal's colors")
	}
}
