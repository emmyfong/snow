package home

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/emmyfong/snow/internal/client/theme"
)

func TestColorProfileKeepsTheConfiguredTheme(t *testing.T) {
	nord, _ := theme.ByName("Nord")
	m := New("v0.1.0", nord)
	m.Update(tea.ColorProfileMsg{Profile: colorprofile.TrueColor})
	if m.theme.Name != "Nord" {
		t.Fatalf("theme %q after the color profile, want Nord", m.theme.Name)
	}
}
