package picker

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func key(code rune, text string) tea.Key { return tea.Key{Code: code, Text: text} }

func TestPreselectsCurrent(t *testing.T) {
	m := New("Split right", []string{"pwsh", "Ubuntu", "cmd"}, "Ubuntu", "/src")
	if r := m.Key(key(tea.KeyEnter, "")); r != Accepted || m.Choice() != "Ubuntu" {
		t.Fatalf("Enter: %v %q, want Accepted Ubuntu", r, m.Choice())
	}
}

func TestMovesAndWraps(t *testing.T) {
	m := New("t", []string{"a", "b", "c"}, "a", "")
	m.Key(key(tea.KeyUp, ""))
	if m.Choice() != "c" {
		t.Fatalf("up from the first: %q, want c", m.Choice())
	}
	m.Key(key('j', "j"))
	m.Key(key(tea.KeyDown, ""))
	if m.Choice() != "b" {
		t.Fatalf("down twice: %q, want b", m.Choice())
	}
}

func TestEscapeCancels(t *testing.T) {
	m := New("t", []string{"a"}, "a", "")
	if r := m.Key(key(tea.KeyEscape, "")); r != Cancelled {
		t.Fatalf("Esc: %v, want Cancelled", r)
	}
}

func TestUnknownCurrentIsListedFirst(t *testing.T) {
	m := New("t", []string{"a"}, "zsh", "")
	if m.Choice() != "zsh" || m.Height() != 4 {
		t.Fatalf("choice %q height %d, want zsh first and 4 rows (2 items, 2 frame rows)", m.Choice(), m.Height())
	}
}

func TestEmptyListCancels(t *testing.T) {
	m := New("t", nil, "", "")
	if r := m.Key(key(tea.KeyEnter, "")); r != Cancelled {
		t.Fatalf("Enter on an empty list: %v, want Cancelled", r)
	}
}
