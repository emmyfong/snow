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
