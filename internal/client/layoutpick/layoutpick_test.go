package layoutpick

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/internal/client/theme"
)

func TestChoose(t *testing.T) {
	m := New()
	m.Key(tea.Key{Code: tea.KeyDown})
	if r := m.Key(tea.Key{Code: tea.KeyEnter}); r != picker.Accepted || m.Choice() != "focus" {
		t.Fatalf("result %v, choice %q", r, m.Choice())
	}
}

// TestLayoutPickerGolden: each preset with what it holds.
func TestLayoutPickerGolden(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			box, _, _ := New().View(theme.Default(), size[0], size[1])
			golden.RequireEqual(t, box)
		})
	}
}
