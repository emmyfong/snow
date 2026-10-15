package keys

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/config"
)

// TestDefaultBindings: every default key maps to its action. These are
// tmux's keys.
func TestDefaultBindings(t *testing.T) {
	tests := []struct {
		key    string
		action string
	}{
		{"%", "split-right"}, {`"`, "split-down"}, {"c", "new-window"},
		{"n", "next-window"}, {"p", "prev-window"},
		{"0", "select-window-0"}, {"1", "select-window-1"}, {"2", "select-window-2"},
		{"3", "select-window-3"}, {"4", "select-window-4"}, {"5", "select-window-5"},
		{"6", "select-window-6"}, {"7", "select-window-7"}, {"8", "select-window-8"},
		{"9", "select-window-9"},
		{",", "rename-window"},
		{"x", "close-pane"}, {"&", "close-window"},
		{"z", "zoom"}, {"E", "even-out"},
		{"Left", "focus-left"}, {"Right", "focus-right"}, {"Up", "focus-up"}, {"Down", "focus-down"},
		{"C-Left", "resize-left"}, {"C-Right", "resize-right"}, {"C-Up", "resize-up"}, {"C-Down", "resize-down"},
		{"M-Left", "resize-left-5"}, {"M-Right", "resize-right-5"}, {"M-Up", "resize-up-5"}, {"M-Down", "resize-down-5"},
		{"d", "detach"},
		{"[", "copy-mode"},
		{"C-b", "send-prefix"},
	}
	tab := Default()
	for _, tt := range tests {
		got, ok := tab.Action(tt.key)
		if !ok || got.Name != tt.action {
			t.Errorf("key %q = %q (bound %v), want %q", tt.key, got.Name, ok, tt.action)
		}
	}
	// No default key is missing from the list above.
	if n := len(tab.Bindings()); n != len(tests) {
		t.Errorf("table holds %d bindings, the test lists %d", n, len(tests))
	}
}

// TestRebind: split-right moves from % to | through config.
func TestRebind(t *testing.T) {
	cfg := config.Defaults().Keys
	cfg.Bind = map[string]string{"|": "split-right", "%": config.UnbindAction}
	tab, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if a, ok := tab.Action("|"); !ok || a.Name != "split-right" {
		t.Errorf("| = %v %v, want split-right", a.Name, ok)
	}
	if a, ok := tab.Action("%"); ok {
		t.Errorf("%% is still bound to %q", a.Name)
	}
	if got := tab.Shortcut("split-right"); got != "Ctrl+b |" {
		t.Errorf("Shortcut(split-right) = %q, want %q", got, "Ctrl+b |")
	}
}

func TestSeveralKeysForOneAction(t *testing.T) {
	cfg := config.Defaults().Keys
	cfg.Bind = map[string]string{"|": "split-right", "C-x": "split-right"}
	tab, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(tab.KeysOf("split-right"), " "); got != "% C-x |" {
		t.Errorf("KeysOf(split-right) = %q, want the default first, then the others by name", got)
	}
	if got := tab.Shortcut("split-right"); got != "Ctrl+b %" {
		t.Errorf("Shortcut = %q, want the first key", got)
	}
}

func TestRebindingAKeyReplacesItsDefault(t *testing.T) {
	cfg := config.Defaults().Keys
	cfg.Bind = map[string]string{"c": "zoom"}
	tab, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if a, _ := tab.Action("c"); a.Name != "zoom" {
		t.Errorf("c = %q, want zoom", a.Name)
	}
	if got := tab.Shortcut("new-window"); got != "" {
		t.Errorf("new-window lost its key but Shortcut = %q", got)
	}
}

func TestPrefix(t *testing.T) {
	tab := Default()
	if !tab.IsPrefix(tea.Key{Code: 'b', Mod: tea.ModCtrl}) {
		t.Error("Ctrl+b is not the default prefix")
	}
	if tab.IsPrefix(tea.Key{Code: 'a', Mod: tea.ModCtrl}) {
		t.Error("Ctrl+a is not the default prefix")
	}
	if got := tab.PrefixLabel(); got != "Ctrl+b" {
		t.Errorf("PrefixLabel = %q", got)
	}

	cfg := config.Defaults().Keys
	cfg.Prefix = "C-a"
	tab, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !tab.IsPrefix(tea.Key{Code: 'a', Mod: tea.ModCtrl}) || tab.IsPrefix(tea.Key{Code: 'b', Mod: tea.ModCtrl}) {
		t.Error("prefix did not move to Ctrl+a")
	}
	// Pressing the prefix twice sends it, so it follows the prefix.
	if a, ok := tab.Action("C-a"); !ok || a.Name != "send-prefix" {
		t.Errorf("C-a = %q, want send-prefix", a.Name)
	}
	if _, ok := tab.Action("C-b"); ok {
		t.Error("the old prefix is still bound")
	}
	if got := tab.Shortcut("zoom"); got != "Ctrl+a z" {
		t.Errorf("Shortcut(zoom) = %q, want %q", got, "Ctrl+a z")
	}
}

func TestPrefixNotes(t *testing.T) {
	cfg := config.Defaults().Keys
	cfg.Prefix = "C-a"
	cfg.Bind = map[string]string{"C-a": "zoom"}
	tab, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if a, _ := tab.Action("C-a"); a.Name != "zoom" {
		t.Errorf("an explicit binding of the prefix key lost to send-prefix: %q", a.Name)
	}
}

func TestLookupNormalizesKeyPresses(t *testing.T) {
	tab := Default()
	tests := []struct {
		name   string
		key    tea.Key
		action string
	}{
		{"text", tea.Key{Code: '%', Text: "%"}, "split-right"},
		{"shifted letter from a real terminal", tea.Key{Code: 'e', ShiftedCode: 'E', Mod: tea.ModShift, Text: "E"}, "even-out"},
		{"digit", tea.Key{Code: '3', Text: "3"}, "select-window-3"},
		{"arrow", tea.Key{Code: tea.KeyLeft}, "focus-left"},
		{"ctrl arrow", tea.Key{Code: tea.KeyUp, Mod: tea.ModCtrl}, "resize-up"},
		{"alt arrow", tea.Key{Code: tea.KeyRight, Mod: tea.ModAlt}, "resize-right-5"},
		{"prefix again", tea.Key{Code: 'b', Mod: tea.ModCtrl}, "send-prefix"},
		{"unbound", tea.Key{Code: 'q', Text: "q"}, ""},
		{"alt letter is not the plain letter", tea.Key{Code: 'c', Mod: tea.ModAlt}, ""},
		{"ctrl letter is not the plain letter", tea.Key{Code: 'c', Mod: tea.ModCtrl}, ""},
	}
	for _, tt := range tests {
		a, ok := tab.Lookup(tt.key)
		if tt.action == "" {
			if ok {
				t.Errorf("%s: bound to %q, want unbound", tt.name, a.Name)
			}
			continue
		}
		if !ok || a.Name != tt.action {
			t.Errorf("%s: %q (bound %v), want %q", tt.name, a.Name, ok, tt.action)
		}
	}
}

func TestNewReportsBadConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     func(*config.Keys)
		wantKey string
		wantIs  error
		wantMsg string
	}{
		{"unknown action", func(k *config.Keys) { k.Bind = map[string]string{"|": "split-sideways"} }, "|", ErrUnknownAction, `unknown action "split-sideways"`},
		{"bad key", func(k *config.Keys) { k.Bind = map[string]string{"C-": "zoom"} }, "C-", ErrBadKey, "keys.bind"},
		{"unknown key name", func(k *config.Keys) { k.Bind = map[string]string{"Hyper": "zoom"} }, "Hyper", ErrBadKey, "Hyper"},
		{"bad prefix", func(k *config.Keys) { k.Prefix = "C-Nope" }, "C-Nope", ErrBadKey, "keys.prefix"},
	}
	for _, tt := range tests {
		cfg := config.Defaults().Keys
		tt.cfg(&cfg)
		_, err := New(cfg)
		var ke *Error
		if !errors.As(err, &ke) {
			t.Errorf("%s: error %v (%T), want *Error", tt.name, err, err)
			continue
		}
		if ke.Key != tt.wantKey || !errors.Is(err, tt.wantIs) || !strings.Contains(err.Error(), tt.wantMsg) {
			t.Errorf("%s: %+v / %q, want key %q, %v, %q", tt.name, ke, err, tt.wantKey, tt.wantIs, tt.wantMsg)
		}
	}
}

// TestEveryDefaultActionExists keeps the defaults and the registry in step.
func TestEveryDefaultActionExists(t *testing.T) {
	for _, b := range Default().Bindings() {
		if b.Action.Name == "" || b.Action.Desc == "" {
			t.Errorf("binding %q has no action name or description: %+v", b.Key, b.Action)
		}
	}
	seen := map[string]bool{}
	for _, a := range Actions() {
		if seen[a.Name] {
			t.Errorf("action %q is listed twice", a.Name)
		}
		seen[a.Name] = true
		if a.Desc == "" {
			t.Errorf("action %q has no description", a.Name)
		}
	}
}
