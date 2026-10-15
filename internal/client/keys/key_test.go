package keys

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestParseKey(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"%", "%"}, {"E", "E"}, {"e", "e"}, {"0", "0"}, {"|", "|"}, {"-", "-"}, {"+", "+"},
		{"C-b", "C-b"}, {"C-B", "C-b"}, {"Ctrl+b", "C-b"}, {"ctrl+a", "C-a"},
		{"M-x", "M-x"}, {"Alt+x", "M-x"},
		{"C--", "C--"},
		{"Left", "Left"}, {"left", "Left"}, {"C-Left", "C-Left"}, {"Ctrl+Left", "C-Left"},
		{"M-Left", "M-Left"}, {"S-Left", "S-Left"}, {"C-M-Left", "C-M-Left"}, {"M-C-Left", "C-M-Left"},
		{"PageUp", "PageUp"}, {"PgUp", "PageUp"}, {"Space", "Space"}, {"Enter", "Enter"},
		{"Esc", "Escape"}, {"BSpace", "BSpace"}, {"F5", "F5"}, {"C-F5", "C-F5"},
		{"S-a", "A"},
	}
	for _, tt := range tests {
		got, err := ParseKey(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("ParseKey(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
		}
	}
	for _, in := range []string{"", "C-", "Hyper", "C-Nope", "ab", "C-M-"} {
		if got, err := ParseKey(in); err == nil {
			t.Errorf("ParseKey(%q) = %q, want an error", in, got)
		}
	}
}

func TestDisplayKey(t *testing.T) {
	tests := map[string]string{
		"%": "%", "C-b": "Ctrl+b", "M-Left": "Alt+Left", "C-M-Left": "Ctrl+Alt+Left", "S-Left": "Shift+Left",
		"Space": "Space", "C--": "Ctrl+-",
	}
	for in, want := range tests {
		if got := DisplayKey(in); got != want {
			t.Errorf("DisplayKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNameOfKeyPress(t *testing.T) {
	tests := []struct {
		key  tea.Key
		want string
	}{
		{tea.Key{Code: 'a', Text: "a"}, "a"},
		{tea.Key{Code: ' ', Text: " "}, "Space"},
		{tea.Key{Code: tea.KeySpace, Text: " "}, "Space"},
		{tea.Key{Code: 'b', Mod: tea.ModCtrl}, "C-b"},
		{tea.Key{Code: 'x', Mod: tea.ModAlt}, "M-x"},
		{tea.Key{Code: tea.KeyF5}, "F5"},
		{tea.Key{Code: tea.KeyLeft, Mod: tea.ModCtrl | tea.ModAlt}, "C-M-Left"},
		{tea.Key{Code: tea.KeyLeft, Mod: tea.ModShift}, "S-Left"},
		{tea.Key{Code: tea.KeyEnter}, "Enter"},
		{tea.Key{Code: tea.KeyEscape}, "Escape"},
		{tea.Key{Code: tea.KeyBackspace}, "BSpace"},
		{tea.Key{Code: tea.KeyPgUp}, "PageUp"},
		{tea.Key{Code: tea.KeyCapsLock}, ""},
	}
	for _, tt := range tests {
		if got := Name(tt.key); got != tt.want {
			t.Errorf("Name(%+v) = %q, want %q", tt.key, got, tt.want)
		}
	}
}
