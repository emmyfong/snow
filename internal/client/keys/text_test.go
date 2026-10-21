package keys

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestTyped(t *testing.T) {
	tests := []struct {
		name string
		key  tea.Key
		want string
		ok   bool
	}{
		{"plain", tea.Key{Code: 'a', Text: "a"}, "a", true},
		{"shift", tea.Key{Code: 'A', Text: "A", Mod: tea.ModShift}, "A", true},
		{"altgr", tea.Key{Code: '/', Text: "/", Mod: tea.ModCtrl | tea.ModAlt}, "/", true},
		{"ctrl", tea.Key{Code: 'a', Text: "a", Mod: tea.ModCtrl}, "", false},
		{"alt", tea.Key{Code: 'x', Text: "x", Mod: tea.ModAlt}, "", false},
		{"no text", tea.Key{Code: tea.KeyUp}, "", false},
		{"ctrl+alt without text", tea.Key{Code: 'x', Mod: tea.ModCtrl | tea.ModAlt}, "", false},
		{"altgr with shift", tea.Key{Code: '|', Text: "|", Mod: tea.ModCtrl | tea.ModAlt | tea.ModShift}, "|", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Typed(tt.key)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("Typed = %q, %v; want %q, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestEdit(t *testing.T) {
	tests := []struct {
		name string
		s    string
		key  tea.Key
		want string
		ok   bool
	}{
		{"type", "ab", tea.Key{Code: 'c', Text: "c"}, "abc", true},
		{"altgr", "a", tea.Key{Code: '@', Text: "@", Mod: tea.ModCtrl | tea.ModAlt}, "a@", true},
		{"backspace", "aé", tea.Key{Code: tea.KeyBackspace}, "a", true},
		{"backspace on empty", "", tea.Key{Code: tea.KeyBackspace}, "", true},
		{"ctrl+u", "abc", tea.Key{Code: 'u', Mod: tea.ModCtrl}, "", true},
		{"enter is not an edit", "abc", tea.Key{Code: tea.KeyEnter}, "abc", false},
		{"ctrl+a is not an edit", "abc", tea.Key{Code: 'a', Mod: tea.ModCtrl}, "abc", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Edit(tt.s, tt.key)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("Edit(%q) = %q, %v; want %q, %v", tt.s, got, ok, tt.want, tt.ok)
			}
		})
	}
}
