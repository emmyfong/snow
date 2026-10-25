package session

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/pkg/api"
)

// chatScreen returns a screen of cols by rows with a terminal pane on the
// left and a chat pane, focused, on the right.
func chatScreen(t *testing.T, cols, rows int) (*Model, *recorder) {
	t.Helper()
	r := &recorder{}
	m := New(r.send)
	m.now = fixedClock
	m.SetSize(cols, rows)
	left := cols / 2
	m.Update(&api.Layout{Session: "work", Active: 1, Windows: []api.WindowInfo{{Index: 1, Name: "sh", Panes: []api.PaneInfo{
		{ID: 1, Kind: api.PaneTerminal, Rect: api.Rect{W: left, H: rows - 1}, Profile: "sh"},
		{ID: 2, Kind: api.PaneChat, Rect: api.Rect{X: left + 1, W: cols - left - 1, H: rows - 1}, Profile: "sh", Focused: true},
	}}}})
	m.Update(&api.PaneUpdate{Pane: 1, Lines: []api.LineUpdate{{Row: 0, Text: "$ go test ./..."}, {Row: 1, Text: "ok  	snow/internal/server"}}})
	m.Update(&api.ChatSnapshot{Pane: 2, Agent: "c1", Conversation: "c1", Model: "anthropic/claude-sonnet-5-5", Mode: "ask", State: api.ChatStateIdle})
	return m, r
}

func chatEvent(e api.ChatEvent) *api.ChatEvent { e.Pane, e.Agent = 2, "c1"; return &e }

func lastChatInput(r *recorder) *api.ChatInput {
	for i := len(r.sent) - 1; i >= 0; i-- {
		if in, ok := r.sent[i].(*api.ChatInput); ok {
			return in
		}
	}
	return nil
}

// TestSessionRoutesKeysToChat: keys typed in a focused chat go to the chat,
// not to api.Input; Enter sends a ChatInput; Ctrl+b d still detaches.
func TestSessionRoutesKeysToChat(t *testing.T) {
	m, r := chatScreen(t, 100, 30)
	for _, k := range []tea.KeyPressMsg{press('h', 0, "h"), press('i', 0, "i"), press(tea.KeyEnter, 0, "")} {
		m.Update(k)
	}
	if len(r.keys()) != 0 {
		t.Fatalf("keys reached a terminal: %v", r.keys())
	}
	if in := lastChatInput(r); in == nil || in.Text != "hi" || in.Pane != 2 || in.Agent != "c1" {
		t.Fatalf("sent %+v", in)
	}
	m.Update(tea.PasteMsg{Content: "pasted"})
	if len(r.keys()) != 0 || !strings.Contains(ansi.Strip(content(m)), "pasted") {
		t.Fatal("paste did not reach the chat input")
	}
	m.Update(ctrlB)
	cmd := m.Update(press('d', 0, "d"))
	if cmd == nil {
		t.Fatal("Ctrl+b d did nothing in a chat")
	}
	if _, ok := cmd().(DetachMsg); !ok {
		t.Fatal("Ctrl+b d did not detach")
	}
}

// TestSessionChatStreams: events after the snapshot draw in the chat, and a
// running turn keeps a spinner tick going.
func TestSessionChatStreams(t *testing.T) {
	m, r := chatScreen(t, 100, 30)
	cmd := m.Update(chatEvent(api.ChatEvent{Kind: api.EventTurnStarted, Turn: 1, Text: "hi"}))
	if cmd == nil {
		t.Fatal("no spinner tick while a turn runs")
	}
	m.Update(chatEvent(api.ChatEvent{Kind: api.EventText, Text: "hello there"}))
	if s := ansi.Strip(content(m)); !strings.Contains(s, "hello there") || !strings.Contains(s, "Esc interrupt") {
		t.Fatalf("screen:\n%s", s)
	}
	m.Update(press(tea.KeyEscape, 0, ""))
	if in := lastChatInput(r); in == nil || in.Kind != api.InputInterrupt {
		t.Fatalf("Esc sent %+v", in)
	}
}

// TestSessionNewChat: Ctrl+b a, the menu item, and the chat command each
// split the focused pane with a chat.
func TestSessionNewChat(t *testing.T) {
	m, r := newPrefixScreen(t)
	m.Update(ctrlB)
	m.Update(press('a', 0, "a"))
	c := lastCommand(t, r)
	if c.Op != api.OpSplit || c.Args[api.ArgKind] != api.PaneChat || c.Args[api.ArgDir] != api.DirRight {
		t.Fatalf("sent %+v", c)
	}
}

// TestChatPaneSkipsCopyMode: copy mode does not open over a chat pane; the
// wheel scrolls the chat instead.
func TestChatPaneSkipsCopyMode(t *testing.T) {
	m, r := chatScreen(t, 100, 30)
	m.Update(ctrlB)
	m.Update(press('[', 0, "["))
	if m.copy != nil {
		t.Fatal("copy mode opened over a chat")
	}
	m.Update(tea.MouseWheelMsg(tea.Mouse{X: 80, Y: 5, Button: tea.MouseWheelUp}))
	if m.copy != nil {
		t.Fatal("the wheel opened copy mode over a chat")
	}
	for _, s := range r.sent {
		if _, ok := s.(*api.ScrollbackRequest); ok {
			t.Fatal("a chat asked for terminal history")
		}
	}
}

// TestSessionDropsGoneChats: a layout without the chat pane forgets it.
func TestSessionDropsGoneChats(t *testing.T) {
	m, _ := chatScreen(t, 100, 30)
	m.Update(&api.Layout{Session: "work", Active: 1, Windows: []api.WindowInfo{{Index: 1, Panes: []api.PaneInfo{{ID: 1, Rect: api.Rect{W: 100, H: 29}, Focused: true}}}}})
	if len(m.chats) != 0 {
		t.Fatalf("chats %v", m.chats)
	}
}

func chatSessionGolden(t *testing.T, cols, rows int) {
	m, _ := chatScreen(t, cols, rows)
	m.Update(chatEvent(api.ChatEvent{Kind: api.EventTurnStarted, Turn: 1, Text: "Why did the server test fail?"}))
	m.Update(chatEvent(api.ChatEvent{Kind: api.EventText, Text: "It passed on the second run. The first run timed out while the shell started."}))
	m.Update(chatEvent(api.ChatEvent{Kind: api.EventTurnEnded, Turn: 1, Reason: api.EndDone}))
	for _, r := range "run it again" {
		m.Update(press(r, 0, string(r)))
	}
	golden.RequireEqual(t, m.View().Content)
}

func TestChatSessionGolden80x24(t *testing.T)  { chatSessionGolden(t, 80, 24) }
func TestChatSessionGolden120x40(t *testing.T) { chatSessionGolden(t, 120, 40) }

// TestChatAskShowsWhatToDo: choosing Log in after a rejected key sends
// nothing and tells the user how to save a key.
func TestChatAskShowsWhatToDo(t *testing.T) {
	m, r := chatScreen(t, 100, 30)
	m.Update(chatEvent(api.ChatEvent{Kind: api.EventTurnStarted, Turn: 1, Text: "hi"}))
	m.Update(chatEvent(api.ChatEvent{Kind: api.EventTurnEnded, Turn: 1, Reason: api.EndFailed,
		Error: &api.ChatError{Code: api.ChatErrBadKey, Message: "key rejected", Choices: []string{api.ChatChoiceLogin}}}))
	sent := len(r.sent)
	if cmd := m.Update(press(tea.KeyEnter, 0, "")); cmd == nil {
		t.Fatal("no message")
	}
	if len(r.sent) != sent {
		t.Fatalf("sent %+v", r.sent[sent:])
	}
	if s := ansi.Strip(content(m)); !strings.Contains(s, "snow auth") {
		t.Fatalf("screen:\n%s", s)
	}
}
