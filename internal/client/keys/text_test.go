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

// TestEditAt: each editing key at the start, middle, and end, and a key
// that does not edit. The text passed in is never changed.
func TestEditAt(t *testing.T) {
	tests := []struct {
		name       string
		cursor     int
		key        tea.Key
		want       string
		wantCursor int
		ok         bool
	}{
		{"left", 2, tea.Key{Code: tea.KeyLeft}, "abc", 1, true},
		{"left at start", 0, tea.Key{Code: tea.KeyLeft}, "abc", 0, true},
		{"right at end", 3, tea.Key{Code: tea.KeyRight}, "abc", 3, true},
		{"home", 2, tea.Key{Code: tea.KeyHome}, "abc", 0, true},
		{"end", 0, tea.Key{Code: tea.KeyEnd}, "abc", 3, true},
		{"backspace", 2, tea.Key{Code: tea.KeyBackspace}, "ac", 1, true},
		{"backspace at start", 0, tea.Key{Code: tea.KeyBackspace}, "abc", 0, true},
		{"delete", 1, tea.Key{Code: tea.KeyDelete}, "ac", 1, true},
		{"delete at end", 3, tea.Key{Code: tea.KeyDelete}, "abc", 3, true},
		{"ctrl+u", 2, tea.Key{Code: 'u', Mod: tea.ModCtrl}, "", 0, true},
		{"typed", 1, tea.Key{Code: 'é', Text: "é"}, "aébc", 2, true},
		{"cursor past end", 9, tea.Key{Code: 'x', Text: "x"}, "abcx", 4, true},
		{"not an edit", 1, tea.Key{Code: tea.KeyUp}, "abc", 1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := []rune("abc")
			got, cur, ok := EditAt(in, tt.cursor, tt.key)
			if string(got) != tt.want || cur != tt.wantCursor || ok != tt.ok {
				t.Fatalf("EditAt = %q, %d, %v; want %q, %d, %v", string(got), cur, ok, tt.want, tt.wantCursor, tt.ok)
			}
			if string(in) != "abc" {
				t.Fatalf("the text passed in changed to %q", string(in))
			}
		})
	}
}
