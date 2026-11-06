package session

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/pkg/api"
)

// recorder collects what a session screen sends to the server.
type recorder struct{ sent []api.Message }

func (r *recorder) send(m api.Message) { r.sent = append(r.sent, m) }

func (r *recorder) keys() []string {
	var out []string
	for _, m := range r.sent {
		if in, ok := m.(*api.Input); ok && in.Key != nil {
			k := in.Key.Code
			if len(in.Key.Mod) > 0 {
				k = in.Key.Mod[0] + "+" + k
			}
			out = append(out, k)
		}
	}
	return out
}

func newPrefixScreen(t *testing.T) (*Model, *recorder) {
	t.Helper()
	r := &recorder{}
	m := New(r.send)
	m.SetSize(80, 24)
	m.Update(&api.Layout{Session: "work", Active: 1, Windows: []api.WindowInfo{{Index: 1, Panes: []api.PaneInfo{{ID: 7, Rect: api.Rect{W: 80, H: 24}}}}}})
	return m, r
}

func press(code rune, mod tea.KeyMod, text string) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code, Mod: mod, Text: text})
}

var ctrlB = press('b', tea.ModCtrl, "")

func TestKeysGoToThePane(t *testing.T) {
	m, r := newPrefixScreen(t)
	m.Update(press('l', 0, "l"))
	m.Update(press('s', 0, "s"))
	if got := r.keys(); len(got) != 2 || got[0] != "l" || got[1] != "s" {
		t.Fatalf("sent %q, want [l s]", got)
	}
	if in := r.sent[0].(*api.Input); in.Pane != 7 {
		t.Fatalf("input went to pane %d, want 7", in.Pane)
	}
}

func TestPrefixDDetaches(t *testing.T) {
	m, r := newPrefixScreen(t)
	m.Update(ctrlB)
	cmd := m.Update(press('d', 0, "d"))
	if cmd == nil {
		t.Fatal("Ctrl+b d returned no command")
	}
	if _, ok := cmd().(DetachMsg); !ok {
		t.Fatal("Ctrl+b d did not ask to detach")
	}
	if len(r.sent) != 0 {
		t.Fatalf("the prefix sequence reached the pane: %v", r.keys())
	}
}

func TestPrefixTwiceSendsLiteralPrefix(t *testing.T) {
	m, r := newPrefixScreen(t)
	m.Update(ctrlB)
	m.Update(ctrlB)
	if got := r.keys(); len(got) != 1 || got[0] != "ctrl+b" {
		t.Fatalf("sent %q, want one ctrl+b", got)
	}
}

func TestPrefixThenUnboundKeyDoesNothing(t *testing.T) {
	m, r := newPrefixScreen(t)
	m.Update(ctrlB)
	m.Update(press('q', 0, "q"))
	m.Update(press('x', 0, "x"))
	if got := r.keys(); len(got) != 1 || got[0] != "x" {
		t.Fatalf("sent %q, want only the key after the unbound one: [x]", got)
	}
}

func TestPasteGoesToThePane(t *testing.T) {
	m, r := newPrefixScreen(t)
	m.Update(tea.PasteMsg{Content: "echo hi"})
	if len(r.sent) != 1 || r.sent[0].(*api.Input).Paste != "echo hi" {
		t.Fatalf("sent %#v, want one paste", r.sent)
	}
}

// TestPrefixClosesAPopup: Ctrl+b with a popup open closes it and starts
// the prefix, so the prefix keys work whatever is open (#287).
func TestPrefixClosesAPopup(t *testing.T) {
	m, r := chatScreen(t, 100, 30)
	typeSlash(m, "/login")
	if m.authPick == nil {
		t.Fatal("/login did not open the login picker")
	}
	m.Update(ctrlB)
	if m.modal() || !m.prefixed {
		t.Fatalf("Ctrl+b over the login picker: modal %v, prefixed %v", m.modal(), m.prefixed)
	}
	cmd := m.Update(press('d', 0, "d"))
	if cmd == nil {
		t.Fatal("Ctrl+b d returned no command")
	}
	if _, ok := cmd().(DetachMsg); !ok {
		t.Fatal("Ctrl+b d did not ask to detach")
	}
	if got := r.keys(); len(got) != 0 {
		t.Fatalf("the prefix reached the pane: %q", got)
	}
}

// TestPrefixClosesEveryPopup: each popup a prefix key opens closes on
// Ctrl+b, which then waits for the next key.
func TestPrefixClosesEveryPopup(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  tea.KeyPressMsg
	}{
		{"new window picker", press('c', 0, "c")},
		{"session list", press('s', 0, "s")},
		{"command prompt", press(':', 0, ":")},
		{"close question", press('x', 0, "x")},
		{"rename", press(',', 0, ",")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := newPrefixScreen(t)
			m.Update(&api.SessionList{Sessions: []api.SessionInfo{{Name: "work"}, {Name: "other"}}})
			m.Update(ctrlB)
			m.Update(tc.key)
			if !m.modal() {
				t.Fatalf("Ctrl+b %s opened no popup", tc.key)
			}
			m.Update(ctrlB)
			if m.modal() || !m.prefixed {
				t.Fatalf("Ctrl+b over the %s: modal %v, prefixed %v", tc.name, m.modal(), m.prefixed)
			}
		})
	}
}

// TestPrefixClosesChatPopups: the popups over a chat close on Ctrl+b too,
// including the login picker's key field, which takes two Escapes, and the
// right-click menu, which the mouse opens.
func TestPrefixClosesChatPopups(t *testing.T) {
	for _, tc := range []struct {
		name string
		open func(m *Model)
	}{
		{"model picker", func(m *Model) { typeSlash(m, "/model"); m.Update(modelList()) }},
		{"help", func(m *Model) { slash(m, "/help") }},
		{"login key field", func(m *Model) {
			typeSlash(m, "/login")
			m.Update(&api.AuthList{Providers: []api.ProviderAuth{{Name: "anthropic", Title: "Anthropic"}}})
			m.Update(press(tea.KeyEnter, 0, ""))
			m.Update(press('s', 0, "s"))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := chatScreen(t, 100, 30)
			tc.open(m)
			if !m.modal() {
				t.Fatalf("%s did not open", tc.name)
			}
			m.Update(ctrlB)
			if m.modal() || !m.prefixed {
				t.Fatalf("Ctrl+b over the %s: modal %v, prefixed %v", tc.name, m.modal(), m.prefixed)
			}
		})
	}
}

// TestPrefixClosesTheMenu: the right-click menu, which the mouse opens,
// closes on Ctrl+b.
func TestPrefixClosesTheMenu(t *testing.T) {
	m, _ := threePaneScreen(t)
	m.Update(rightClick(50, 3))
	if m.menu == nil {
		t.Fatal("a right click opened no menu")
	}
	m.Update(ctrlB)
	if m.modal() || !m.prefixed {
		t.Fatalf("Ctrl+b over the menu: modal %v, prefixed %v", m.modal(), m.prefixed)
	}
}
