package session

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/pkg/api"
)

var testKinds = &api.PaneKinds{Kinds: []api.PaneKindInfo{
	{Name: api.PaneTerminal, Title: "Terminal"},
	{Name: api.PaneChat, Title: "Chat"},
	{Name: "files", Title: "Files"},
}}

func wantOpen(t *testing.T, r *recorder, target int, kind string) {
	t.Helper()
	c := lastCommand(t, r)
	if c.Op != api.OpOpen || c.Target != target || c.Args[api.ArgKind] != kind {
		t.Fatalf("sent %+v, want open %s on pane %d", c, kind, target)
	}
}

// TestChatOpenSendsCommand: /open with a kind in a chat opens it beside
// the chat.
func TestChatOpenSendsCommand(t *testing.T) {
	m, r := chatScreen(t, 100, 30)
	typeIn(m, "/open Files")
	m.Update(enter)
	wantOpen(t, r, 2, "files")
}

// TestOpenPicker: /open alone in a chat, the session's open command, and
// Ctrl+b O each show the kind picker; Enter opens the chosen kind.
func TestOpenPicker(t *testing.T) {
	tests := []struct {
		name string
		show func(t *testing.T, m *Model)
	}{
		{"chat command", func(_ *testing.T, m *Model) { typeIn(m, "/open"); m.Update(enter) }},
		{"session command", func(_ *testing.T, m *Model) {
			m.Update(ctrlB)
			m.Update(press(':', 0, ":"))
			typeIn(m, "open")
			m.Update(enter)
		}},
		{"key", func(_ *testing.T, m *Model) { m.Update(ctrlB); m.Update(press('O', tea.ModShift, "O")) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, r := chatScreen(t, 100, 30)
			m.Update(testKinds)
			tt.show(t, m)
			if m.over == nil {
				t.Fatal("no kind picker")
			}
			m.Update(press(tea.KeyDown, 0, ""))
			m.Update(press(tea.KeyDown, 0, ""))
			m.Update(enter)
			wantOpen(t, r, 2, "files")
			if m.over != nil {
				t.Fatal("the picker stayed open")
			}
		})
	}
}

// TestMenuOpensKindPicker: the pane menu's Open pane item shows the kind
// picker for the pane that was clicked.
func TestMenuOpensKindPicker(t *testing.T) {
	m, r := pickerScreen(t, 80, 24)
	m.Update(testKinds)
	chooseByClick(t, m, "Open pane...")
	if m.over == nil {
		t.Fatal("no kind picker")
	}
	m.Update(press(tea.KeyDown, 0, ""))
	m.Update(press(tea.KeyDown, 0, ""))
	m.Update(enter)
	if c := lastCommand(t, r); c.Op != api.OpOpen || c.Args[api.ArgKind] != "files" {
		t.Fatalf("sent %+v, want open files", c)
	}
}

// TestOpenPickerNoKinds: a server that lists no kinds gets an error, not
// an empty picker.
func TestOpenPickerNoKinds(t *testing.T) {
	m, _ := chatScreen(t, 100, 30)
	m.openKindPicker(2)
	if m.over != nil {
		t.Fatal("an empty kind picker opened")
	}
}

// TestKindsInWindow: the picker marks every kind of the window's panes;
// a pane with no kind is a terminal.
func TestKindsInWindow(t *testing.T) {
	m, _ := chatScreen(t, 100, 30)
	got := m.kindsInWindow(2)
	if !got[api.PaneTerminal] || !got[api.PaneChat] || got["files"] {
		t.Fatalf("kinds %v", got)
	}
}
