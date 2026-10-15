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
