package keys

import (
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
