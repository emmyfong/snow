package session

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/keys"
	"github.com/emmyfong/snow/internal/config"
	"github.com/emmyfong/snow/pkg/api"
)

// withConfig returns a prefix screen whose key table comes from cfg.
func withConfig(t *testing.T, cfg config.Keys) (*Model, *recorder) {
	t.Helper()
	tab, err := keys.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	r := &recorder{}
	m := New(r.send, WithKeys(tab))
	m.SetSize(80, 24)
	m.Update(&api.Layout{Session: "work", Active: 1, Windows: []api.WindowInfo{{Index: 1, Name: "dev", Panes: []api.PaneInfo{{ID: 7, Rect: api.Rect{W: 80, H: 23}, Profile: "sh"}}}}})
	m.Update(&api.ProfileList{Profiles: []string{"sh"}, Default: "sh"})
	return m, r
}

func TestRebindThroughConfigMovesSplitRight(t *testing.T) {
	cfg := config.Defaults().Keys
	cfg.Prefix = "C-a"
	cfg.Bind = map[string]string{"|": "split-right", "%": config.UnbindAction}
	m, r := withConfig(t, cfg)

	m.Update(press('a', tea.ModCtrl, ""))
	m.Update(press('|', 0, "|"))
	if m.picker == nil || m.pending.Op != api.OpSplit || m.pending.Args[api.ArgDir] != api.DirRight {
		t.Fatalf("Ctrl+a | did not open the split-right picker: picker %v, pending %+v", m.picker != nil, m.pending)
	}
	m.Update(press(tea.KeyEscape, 0, ""))

	// The old prefix is an ordinary key now, and so is the old split key.
	m.Update(ctrlB)
	m.Update(press('%', 0, "%"))
	if got := r.keys(); !slices.Equal(got, []string{"ctrl+b", "%"}) {
		t.Fatalf("sent %q, want the old prefix and the old split key to reach the pane", got)
	}
	m.Update(press('a', tea.ModCtrl, ""))
	m.Update(press('%', 0, "%"))
	if m.picker != nil || len(r.sent) != 2 {
		t.Fatal("the unbound key % did something after the new prefix")
	}
}

func TestNewPrefixTwiceSendsIt(t *testing.T) {
	cfg := config.Defaults().Keys
	cfg.Prefix = "C-a"
	m, r := withConfig(t, cfg)
	m.Update(press('a', tea.ModCtrl, ""))
	m.Update(press('a', tea.ModCtrl, ""))
	if got := r.keys(); !slices.Equal(got, []string{"ctrl+a"}) {
		t.Fatalf("sent %q, want one ctrl+a", got)
	}
}

// TestEveryActionHasAHandler keeps the key table and the screen in step. A
// new action in internal/client/keys needs a handler here.
func TestEveryActionHasAHandler(t *testing.T) {
	for _, a := range keys.Actions() {
		if handlers[a.Name] == nil {
			t.Errorf("action %q has no handler", a.Name)
		}
	}
	for name := range handlers {
		if _, ok := keys.ActionByName(name); !ok {
			t.Errorf("handler %q has no action", name)
		}
	}
}

func TestRenameWindow(t *testing.T) {
	m, r := withConfig(t, config.Defaults().Keys)
	m.Update(ctrlB)
	m.Update(press(',', 0, ","))
	if got := statusLine(m); !strings.Contains(got, "rename window: dev_") {
		t.Fatalf("status %q, want the prompt with the current name", got)
	}
	m.Update(press(tea.KeyBackspace, 0, ""))
	m.Update(press('v', 0, "v"))
	m.Update(press('x', 0, "x"))
	if got := statusLine(m); !strings.Contains(got, "rename window: devx_") {
		t.Fatalf("status %q, want the typed name", got)
	}
	m.Update(press(tea.KeyEnter, 0, ""))
	c := lastCommand(t, r)
	if c.Op != api.OpRenameWindow || c.Target != 1 || c.Args[api.ArgName] != "devx" {
		t.Fatalf("sent %+v, want rename-window 1 to devx", c)
	}
	if strings.Contains(statusLine(m), "rename window") {
		t.Fatal("the prompt stayed after Enter")
	}
}

// TestRenameWindowAltGr: a character typed with AltGr (reported as
// Ctrl+Alt with text on Windows) goes into the name (#84).
func TestRenameWindowAltGr(t *testing.T) {
	m, r := withConfig(t, config.Defaults().Keys)
	m.Update(ctrlB)
	m.Update(press(',', 0, ","))
	m.Update(press('@', tea.ModCtrl|tea.ModAlt, "@"))
	m.Update(press(tea.KeyEnter, 0, ""))
	if c := lastCommand(t, r); c.Args[api.ArgName] != "dev@" {
		t.Fatalf("sent name %q, want dev@", c.Args[api.ArgName])
	}
}

func TestRenameCancelAndEmpty(t *testing.T) {
	m, r := withConfig(t, config.Defaults().Keys)
	m.Update(ctrlB)
	m.Update(press(',', 0, ","))
	m.Update(press('z', 0, "z"))
	m.Update(press(tea.KeyEscape, 0, ""))
	if len(r.sent) != 0 || m.rename != nil {
		t.Fatalf("Escape sent %v or left the prompt open", r.sent)
	}
	m.Update(ctrlB)
	m.Update(press(',', 0, ","))
	m.Update(press('u', tea.ModCtrl, ""))
	m.Update(press(tea.KeyEnter, 0, ""))
	if len(r.sent) != 0 {
		t.Fatalf("an empty name was sent: %v", r.sent)
	}
}

func TestPasteIgnoredWhileAskingOrRenaming(t *testing.T) {
	m, r := withConfig(t, config.Defaults().Keys)
	m.Update(ctrlB)
	m.Update(press(',', 0, ","))
	m.Update(tea.PasteMsg{Content: "hello"})
	if len(r.sent) != 0 {
		t.Fatalf("a paste reached the pane while renaming: %v", r.sent)
	}
}

// rebound is the table of TestRebind: split-right on | instead of %.
func rebound(t *testing.T) *keys.Table {
	t.Helper()
	cfg := config.Defaults().Keys
	cfg.Bind = map[string]string{"|": "split-right", "%": config.UnbindAction}
	tab, err := keys.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return tab
}

// TestSendPrefixSendsThePrefixNotTheKeyThatRanIt: with prefix C-a and the key
// "a" bound to send-prefix, Ctrl+a a must type Ctrl+a.
func TestSendPrefixSendsThePrefixNotTheKeyThatRanIt(t *testing.T) {
	cfg := config.Defaults().Keys
	cfg.Prefix = "C-a"
	cfg.Bind = map[string]string{"a": "send-prefix"}
	m, r := withConfig(t, cfg)
	m.Update(press('a', tea.ModCtrl, ""))
	m.Update(press('a', 0, "a"))
	if got := r.keys(); !slices.Equal(got, []string{"ctrl+a"}) {
		t.Fatalf("sent %q, want one ctrl+a", got)
	}
}
