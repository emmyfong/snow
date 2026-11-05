package session

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/pkg/api"
)

func wantLayout(t *testing.T, r *recorder, target int, name string) {
	t.Helper()
	c := lastCommand(t, r)
	if c.Op != api.OpLayout || c.Target != target || c.Args[api.ArgName] != name {
		t.Fatalf("sent %+v, want layout %s on pane %d", c, name, target)
	}
}

// TestChatLayoutSendsCommand: /layout with a name in a chat arranges the
// chat's window.
func TestChatLayoutSendsCommand(t *testing.T) {
	m, r := chatScreen(t, 100, 30)
	typeIn(m, "/layout IDE")
	m.Update(enter)
	wantLayout(t, r, 2, "ide")
}

// TestLayoutPicker: /layout alone in a chat, the session's layout command,
// and Ctrl+b Space each show the preset picker; Enter applies the choice.
func TestLayoutPicker(t *testing.T) {
	tests := []struct {
		name string
		show func(m *Model)
	}{
		{"chat command", func(m *Model) { typeIn(m, "/layout"); m.Update(enter) }},
		{"session command", func(m *Model) { m.Update(ctrlB); m.Update(press(':', 0, ":")); typeIn(m, "layout"); m.Update(enter) }},
		{"key", func(m *Model) { m.Update(ctrlB); m.Update(press(tea.KeySpace, 0, " ")) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, r := chatScreen(t, 100, 30)
			tt.show(m)
			if m.over == nil {
				t.Fatal("no layout picker")
			}
			m.Update(press(tea.KeyDown, 0, ""))
			m.Update(enter)
			wantLayout(t, r, 2, "focus")
		})
	}
}

// TestMenuOpensLayoutPicker: the pane menu's Layout item shows the preset
// picker.
func TestMenuOpensLayoutPicker(t *testing.T) {
	m, r := pickerScreen(t, 80, 24)
	chooseByClick(t, m, "Layout...")
	if m.over == nil {
		t.Fatal("no layout picker")
	}
	m.Update(enter)
	if c := lastCommand(t, r); c.Op != api.OpLayout || c.Args[api.ArgName] != "ide" {
		t.Fatalf("sent %+v, want layout ide", c)
	}
}
