package keys

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestParseKey(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"%", "%"}, {"E", "E"}, {"e", "e"}, {"0", "0"}, {"|", "|"}, {"-", "-"}, {"+", "+"},
		{"C-b", "C-b"}, {"Ctrl+b", "C-b"}, {"C-S-b", "C-S-b"}, {"ctrl+a", "C-a"},
		{"M-x", "M-x"}, {"Alt+x", "M-x"},
		{"C--", "C--"},
		{"Left", "Left"}, {"left", "Left"}, {"C-Left", "C-Left"}, {"Ctrl+Left", "C-Left"},
		{"M-Left", "M-Left"}, {"S-Left", "S-Left"}, {"C-M-Left", "C-M-Left"}, {"M-C-Left", "C-M-Left"},
		{"PageUp", "PageUp"}, {"PgUp", "PageUp"}, {"Space", "Space"}, {"Enter", "Enter"},
		{"Esc", "Escape"}, {"BSpace", "BSpace"}, {"F5", "F5"}, {"C-F5", "C-F5"},
		{"S-a", "A"}, {"M-S-e", "M-S-e"},
	}
	for _, tt := range tests {
		got, err := ParseKey(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("ParseKey(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
		}
	}
	for _, in := range []string{"", "C-", "Hyper", "C-Nope", "ab", "C-M-", "C-B", "M-E", "Ctrl+B"} {
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

// With Ctrl or Alt, an upper-case letter is ambiguous: terminals report
// Ctrl+b the same with or without Shift. ParseKey refuses it and says to
// write the shift.
func TestParseKeyUpperCaseWithModifierIsAnError(t *testing.T) {
	_, err := ParseKey("M-E")
	if err == nil || !strings.Contains(err.Error(), "M-S-e") {
		t.Fatalf("ParseKey(M-E) error = %v, want a hint to write M-S-e", err)
	}
	if !errors.Is(err, ErrBadKey) {
		t.Fatalf("error %v does not wrap ErrBadKey", err)
	}
}

func TestNameOfAltShiftLetterMatchesParse(t *testing.T) {
	want, _ := ParseKey("M-S-e")
	if got := Name(tea.Key{Code: 'e', ShiftedCode: 'E', Mod: tea.ModAlt | tea.ModShift}); got != want {
		t.Fatalf("Name = %q, want %q", got, want)
	}
}
