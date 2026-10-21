package keys

import tea "charm.land/bubbletea/v2"

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
