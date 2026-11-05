package kindpick

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

var kinds = []api.PaneKindInfo{
	{Name: api.PaneTerminal, Title: "Terminal"},
	{Name: api.PaneChat, Title: "Chat"},
	{Name: "files", Title: "Files"},
}

func TestChoose(t *testing.T) {
	m := New(kinds, nil)
	m.Key(tea.Key{Code: tea.KeyDown})
	m.Key(tea.Key{Code: tea.KeyDown})
	if r := m.Key(tea.Key{Code: tea.KeyEnter}); r != picker.Accepted || m.Choice() != "files" {
		t.Fatalf("result %v, choice %q", r, m.Choice())
	}
}

func TestEmpty(t *testing.T) {
	m := New(nil, nil)
	if c := m.Choice(); c != "" {
		t.Fatalf("choice %q from no kinds", c)
	}
	m.View(theme.Default(), 80, 24)
}

// TestKindPickerGolden: each kind with its name, and the chat marked as
// already in the window.
func TestKindPickerGolden(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			box, _, _ := New(kinds, map[string]bool{api.PaneChat: true}).View(theme.Default(), size[0], size[1])
			golden.RequireEqual(t, box)
		})
	}
}
