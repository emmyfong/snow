package chat

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/pkg/api"
)

// TestLoginCommandAsks: /login asks the session for the login picker and
// sends nothing to the server.
func TestLoginCommandAsks(t *testing.T) {
	m, _ := newChat(80, 24)
	if in := runSlash(m, "/login"); in != nil {
		t.Fatalf("/login sent %+v", in)
	}
	if got := m.Asked(); got != AskLogin {
		t.Fatalf("Asked = %q, want %q", got, AskLogin)
	}
}

// TestLogoutCommandAsks: /logout asks the session to remove a key.
func TestLogoutCommandAsks(t *testing.T) {
	m, _ := newChat(80, 24)
	if in := runSlash(m, "/logout"); in != nil {
		t.Fatalf("/logout sent %+v", in)
	}
	if got := m.Asked(); got != AskLogout {
		t.Fatalf("Asked = %q, want %q", got, AskLogout)
	}
}

// TestNoKeyShowsConnect: an empty chat with no key says how to connect a
// provider; when a key exists it points at /model instead.
func TestNoKeyShowsConnect(t *testing.T) {
	m, _ := newChat(80, 24)
	m.Snapshot(&api.ChatSnapshot{Pane: 2, Agent: "c1", Conversation: "c1", Mode: "ask", State: api.ChatStateIdle})
	if s := screen(m); !strings.Contains(s, "Connect a provider") || !strings.Contains(s, "Type /login to save one.") {
		t.Fatalf("empty chat with no key:\n%s", s)
	}
	m.SetProviderReady(true)
	s := screen(m)
	if strings.Contains(s, "Connect a provider") || !strings.Contains(s, "Type /model to choose one.") {
		t.Fatalf("empty chat with a key:\n%s", s)
	}
}

// TestNoKeyRefusesTheMessage: with no key and no model, a message says to
// connect a provider instead of choosing a model.
func TestNoKeyRefusesTheMessage(t *testing.T) {
	m, _ := newChat(80, 24)
	m.snap.Model = ""
	typeText(m, "hello")
	if in := m.Key(key(tea.KeyEnter, 0, "")); in != nil {
		t.Fatalf("a message without a key sent %+v", in)
	}
	if note := m.Note(); !strings.Contains(note, "Connect a provider") {
		t.Fatalf("note = %q", note)
	}
}
