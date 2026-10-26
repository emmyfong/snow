package modepick

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

func TestStartsOnTheCurrentMode(t *testing.T) {
	for _, tt := range []struct{ current, want string }{
		{api.ChatModeAsk, api.ChatModeAsk},
		{api.ChatModeAuto, api.ChatModeAuto},
		{"", api.ChatModeAsk},
	} {
		if got := New(tt.current).Choice(); got != tt.want {
			t.Errorf("current %q starts on %q, want %q", tt.current, got, tt.want)
		}
	}
}

func TestChoose(t *testing.T) {
	m := New(api.ChatModeAsk)
	m.Key(tea.Key{Code: tea.KeyDown})
	if r := m.Key(tea.Key{Code: tea.KeyEnter}); r != picker.Accepted || m.Choice() != api.ChatModeAuto {
		t.Fatalf("result %v, choice %q", r, m.Choice())
	}
}

// TestModePickerGolden: both modes with a line on what each does, and the
// current one marked.
func TestModePickerGolden(t *testing.T) {
	m := New(api.ChatModeAuto)
	box, _, _ := m.View(theme.Default(), 80, 24)
	golden.RequireEqual(t, box)
}
