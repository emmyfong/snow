package keys

import (
	"slices"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

// Typed returns the text k types, if any. A key with Ctrl or Alt alone is a
// shortcut, not text. Ctrl and Alt together with text is AltGr: Windows
// reports it that way, and layouts such as German type "/", "@", and "{"
// with it.
func Typed(k tea.Key) (string, bool) {
	if k.Text == "" {
		return "", false
	}
	switch k.Mod & (tea.ModCtrl | tea.ModAlt) {
	case 0, tea.ModCtrl | tea.ModAlt:
		return k.Text, true
	}
	return "", false
}

// Edit applies k to a one-line text field holding s: Backspace deletes the
// last character, Ctrl+U clears the field, and typed text is added. It
// reports false for keys that do not edit, so the caller can handle them.
// Callers that limit the length check the result.
func Edit(s string, k tea.Key) (string, bool) {
	switch {
	case k.Code == tea.KeyBackspace:
		_, n := utf8.DecodeLastRuneInString(s)
		return s[:len(s)-n], true
	case k.Code == 'u' && k.Mod == tea.ModCtrl:
		return "", true
	}
	if text, ok := Typed(k); ok {
		return s + text, true
	}
	return s, false
}

// EditAt applies k to a text field holding text with the cursor at cursor,
// a count of runes: Left, Right, Home, and End move the cursor; Backspace
// and Delete remove the character before or after it; Ctrl+U clears the
// field; and typed text goes in at the cursor. It returns the new text and
// cursor, and false for keys that do not edit, so the caller can handle
// them. It never changes text in place.
func EditAt(text []rune, cursor int, k tea.Key) ([]rune, int, bool) {
	cursor = min(max(cursor, 0), len(text))
	switch {
	case k.Code == tea.KeyLeft:
		return text, max(cursor-1, 0), true
	case k.Code == tea.KeyRight:
		return text, min(cursor+1, len(text)), true
	case k.Code == tea.KeyHome:
		return text, 0, true
	case k.Code == tea.KeyEnd:
		return text, len(text), true
	case k.Code == tea.KeyBackspace:
		if cursor == 0 {
			return text, 0, true
		}
		return slices.Delete(slices.Clone(text), cursor-1, cursor), cursor - 1, true
	case k.Code == tea.KeyDelete:
		if cursor == len(text) {
			return text, cursor, true
		}
		return slices.Delete(slices.Clone(text), cursor, cursor+1), cursor, true
	case k.Code == 'u' && k.Mod == tea.ModCtrl:
		return nil, 0, true
	}
	if s, ok := Typed(k); ok {
		text, cursor = InsertAt(text, cursor, s)
		return text, cursor, true
	}
	return text, cursor, false
}

// InsertAt puts s into text at cursor and returns the new text and the
// cursor after s. It never changes text in place.
func InsertAt(text []rune, cursor int, s string) ([]rune, int) {
	cursor = min(max(cursor, 0), len(text))
	r := []rune(s)
	return slices.Insert(slices.Clone(text), cursor, r...), cursor + len(r)
}
