package session

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/pkg/api"
)

// slash types a slash command into the focused chat and presses Enter.
func slash(m *Model, text string) {
	for _, r := range text {
		m.Update(press(r, 0, string(r)))
	}
	m.Update(press(tea.KeyEnter, 0, ""))
}

// sentOf returns the messages of type T the recorder holds.
func sentOf[T api.Message](r *recorder) []T {
	var out []T
	for _, m := range r.sent {
		if v, ok := m.(T); ok {
			out = append(out, v)
		}
	}
	return out
}

func conversations(all bool) *api.ConversationList {
	now := fixedClock()
	return &api.ConversationList{Pane: 2, All: all, Items: []api.ConversationInfo{
		{ID: "c9", Title: "Fix the rename bug", Folder: "/home/u/snow", Model: "anthropic/claude-opus-5-5", LastActivity: now.Add(-5 * time.Minute), Turns: 7},
		{ID: "c8", Title: "Explain the picker", Folder: "/home/u/snow", Model: "openai/gpt-6.1", LastActivity: now.Add(-30 * time.Hour), Turns: 1, Open: true},
	}}
}

// TestResumePickerFlow: /resume asks the server for this folder's list,
// shows it, and Enter sends the choice. Ctrl+a asks for every folder, and
// Esc sends nothing.
func TestResumePickerFlow(t *testing.T) {
	m, r := chatScreen(t, 120, 30)
	slash(m, "/resume")
	if reqs := sentOf[*api.ConversationListRequest](r); len(reqs) != 1 || reqs[0].Pane != 2 || reqs[0].All {
		t.Fatalf("requests %+v", reqs)
	}
	m.Update(conversations(false))
	if c := ansi.Strip(content(m)); !strings.Contains(c, "Fix the rename bug") || !strings.Contains(c, "open") {
		t.Fatalf("the list is not shown:\n%s", c)
	}

	m.Update(press('a', tea.ModCtrl, ""))
	if reqs := sentOf[*api.ConversationListRequest](r); len(reqs) != 2 || !reqs[1].All {
		t.Fatalf("Ctrl+a sent %+v", reqs)
	}
	m.Update(conversations(true))
	m.Update(press(tea.KeyDown, 0, ""))
	m.Update(press(tea.KeyEnter, 0, ""))
	in := lastChatInput(r)
	if in == nil || in.Kind != api.ChatInputResume || in.Conversation != "c8" || in.Pane != 2 || in.Agent != "c1" {
		t.Fatalf("Enter sent %+v", in)
	}
	if m.over != nil || m.resume != nil {
		t.Fatal("the picker stayed open after a choice")
	}

	inputs := len(sentOf[*api.ChatInput](r))
	slash(m, "/resume")
	m.Update(conversations(false))
	m.Update(press(tea.KeyEscape, 0, ""))
	if m.over != nil || len(sentOf[*api.ChatInput](r)) != inputs {
		t.Fatalf("Esc left the picker open or sent a chat input: over=%v", m.over != nil)
	}
}

// TestResumePickerClick: a click on a row resumes that conversation; a
// click outside closes the picker.
func TestResumePickerClick(t *testing.T) {
	m, r := chatScreen(t, 120, 30)
	slash(m, "/resume")
	m.Update(conversations(false))
	box, x, y := m.over.View(m.theme, m.cols, m.windowRows())
	if box == "" {
		t.Fatal("no picker")
	}
	m.Update(tea.MouseClickMsg{X: x + 4, Y: y + 2, Button: tea.MouseLeft})
	if in := lastChatInput(r); in == nil || in.Conversation != "c8" {
		t.Fatalf("click sent %+v", in)
	}
	slash(m, "/resume")
	m.Update(tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft})
	if m.over != nil {
		t.Fatal("a click outside left the picker open")
	}
}

// TestModePickerFlow: /mode opens the picker on the current mode; choosing
// the other sends it.
func TestModePickerFlow(t *testing.T) {
	m, r := chatScreen(t, 100, 30)
	slash(m, "/mode")
	if c := ansi.Strip(content(m)); !strings.Contains(c, "Approval mode") || !strings.Contains(c, "(current)") {
		t.Fatalf("no mode picker:\n%s", c)
	}
	m.Update(press(tea.KeyDown, 0, ""))
	m.Update(press(tea.KeyEnter, 0, ""))
	in := lastChatInput(r)
	if in == nil || in.Kind != api.ChatInputMode || in.Mode != api.ChatModeAuto || in.Pane != 2 || in.Agent != "c1" {
		t.Fatalf("sent %+v", in)
	}
	if m.over != nil {
		t.Fatal("the picker stayed open")
	}
}

// TestHelpPanel: /help lists the chat's commands and keys; any key closes
// it and reaches nothing else.
func TestHelpPanel(t *testing.T) {
	m, r := chatScreen(t, 100, 30)
	slash(m, "/help")
	c := ansi.Strip(content(m))
	for _, want := range []string{"Help", "/resume", "/rename [title]", "/mode [ask|auto]", "/effort [level]", "Esc", "Press any key to close."} {
		if !strings.Contains(c, want) {
			t.Errorf("help lacks %q:\n%s", want, c)
		}
	}
	before := len(r.sent)
	m.Update(press('x', 0, "x"))
	if m.over != nil || len(r.sent) != before {
		t.Fatalf("the key was not swallowed: over=%v, %d messages after %d", m.over != nil, len(r.sent), before)
	}
}

// TestOverlayClosesWithItsPane: a layout without the chat pane closes the
// overlay, so a choice never goes to a pane that is gone.
func TestOverlayClosesWithItsPane(t *testing.T) {
	m, _ := chatScreen(t, 100, 30)
	slash(m, "/mode")
	m.Update(&api.Layout{Session: "work", Active: 1, Windows: []api.WindowInfo{{Index: 1, Name: "sh", Panes: []api.PaneInfo{
		{ID: 1, Kind: api.PaneTerminal, Rect: api.Rect{W: 100, H: 29}, Profile: "sh", Focused: true},
	}}}})
	if m.over != nil {
		t.Fatal("the overlay outlived its pane")
	}
}

// TestSmallWindowRefusesOverlay: a window too small says so instead of
// holding the keyboard for a popup nobody sees.
func TestSmallWindowRefusesOverlay(t *testing.T) {
	m, _ := chatScreen(t, 24, 8)
	slash(m, "/mode")
	if m.over != nil {
		t.Fatal("an overlay opened in a window too small for it")
	}
}

// TestHelpGolden: the chat's help, over the chat.
func TestHelpGolden(t *testing.T) {
	m, _ := chatScreen(t, 100, 30)
	slash(m, "/help")
	golden.RequireEqual(t, m.View().Content)
}

// TestResumeOverlayGolden: the resume picker over the chat.
func TestResumeOverlayGolden(t *testing.T) {
	m, _ := chatScreen(t, 120, 30)
	slash(m, "/resume")
	m.Update(conversations(false))
	golden.RequireEqual(t, m.View().Content)
}
