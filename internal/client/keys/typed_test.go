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
