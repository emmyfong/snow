package keys

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

// Keys have one canonical spelling, in tmux's style: modifiers C-, M-, and
// S- in that order, then a single character or a name such as Left, F5, or
// Space. ParseKey turns anything the user may write into it; Name turns a
// key press into it, so the two can be compared as strings.

// names maps Bubble Tea's special key codes to canonical names.
var names = map[rune]string{
	tea.KeyUp: "Up", tea.KeyDown: "Down", tea.KeyLeft: "Left", tea.KeyRight: "Right",
	tea.KeyHome: "Home", tea.KeyEnd: "End", tea.KeyPgUp: "PageUp", tea.KeyPgDown: "PageDown",
	tea.KeyInsert: "Insert", tea.KeyDelete: "Delete", tea.KeyEnter: "Enter", tea.KeyTab: "Tab",
	tea.KeySpace: "Space", tea.KeyEscape: "Escape", tea.KeyBackspace: "BSpace",
	tea.KeyF1: "F1", tea.KeyF2: "F2", tea.KeyF3: "F3", tea.KeyF4: "F4", tea.KeyF5: "F5", tea.KeyF6: "F6",
	tea.KeyF7: "F7", tea.KeyF8: "F8", tea.KeyF9: "F9", tea.KeyF10: "F10", tea.KeyF11: "F11", tea.KeyF12: "F12",
}

// aliases maps lower-case spellings in config files to canonical names.
var aliases = map[string]string{}

func init() {
	for _, n := range names {
		aliases[strings.ToLower(n)] = n
	}
	for alias, n := range map[string]string{
		"pgup": "PageUp", "ppage": "PageUp", "pgdn": "PageDown", "pgdown": "PageDown", "npage": "PageDown",
		"return": "Enter", "esc": "Escape", "backspace": "BSpace", "ic": "Insert", "dc": "Delete",
	} {
		aliases[alias] = n
	}
}

// mods are the modifier prefixes a key may carry, in canonical order.
var mods = []struct {
	letter  string
	display string
	words   []string // spellings with +, lower case
}{
	{"C-", "Ctrl+", []string{"ctrl+", "control+"}},
	{"M-", "Alt+", []string{"alt+", "meta+"}},
	{"S-", "Shift+", []string{"shift+"}},
}

// ParseKey converts a key as written in the config file, for example "C-b",
// "Ctrl+b", "M-Left", or "%", to its canonical spelling.
func ParseKey(s string) (string, error) {
	rest := s
	var has [3]bool
	for {
		matched := false
		for i, m := range mods {
			switch {
			case len(rest) > len(m.letter) && strings.EqualFold(rest[:len(m.letter)], m.letter):
				rest = rest[len(m.letter):]
			case hasWord(rest, m.words):
				rest = rest[wordLen(rest, m.words):]
			default:
				continue
			}
			has[i], matched = true, true
			break
		}
		if !matched {
			break
		}
	}
	if rest == "" {
		return "", fmt.Errorf("%w %q: nothing after the modifier", ErrBadKey, s)
	}
	base := rest
	if utf8.RuneCountInString(rest) == 1 {
		r, _ := utf8.DecodeRuneInString(rest)
		if !unicode.IsPrint(r) || unicode.IsSpace(r) {
			return "", fmt.Errorf("%w %q", ErrBadKey, s)
		}
		// Terminals report Ctrl+b the same with or without Shift, so an
		// upper-case letter here would bind a key that may never arrive.
		if (has[0] || has[1]) && unicode.IsUpper(r) {
			return "", fmt.Errorf("%w %q: write the shift as S-, for example %q", ErrBadKey, s,
				canonical(has[0], has[1], true, strings.ToLower(rest)))
		}
	} else if n, ok := aliases[strings.ToLower(rest)]; ok {
		base = n
	} else {
		return "", fmt.Errorf("%w %q", ErrBadKey, s)
	}
	return canonical(has[0], has[1], has[2], base), nil
}

func hasWord(s string, words []string) bool {
	return wordLen(s, words) > 0 && len(s) > wordLen(s, words)
}

func wordLen(s string, words []string) int {
	for _, w := range words {
		if len(s) >= len(w) && strings.EqualFold(s[:len(w)], w) {
			return len(w)
		}
	}
	return 0
}

// canonical joins modifiers and a base key. A single character with Shift
// and no other modifier is the upper-case character; with Ctrl or Alt the
// character is lower case and Shift stays a modifier: C-S-b.
func canonical(ctrl, alt, shift bool, base string) string {
	if utf8.RuneCountInString(base) == 1 {
		if ctrl || alt {
			base = strings.ToLower(base)
		} else if shift {
			base, shift = strings.ToUpper(base), false
		}
	}
	var b strings.Builder
	for i, on := range [3]bool{ctrl, alt, shift} {
		if on {
			b.WriteString(mods[i].letter)
		}
	}
	return b.String() + base
}

// Name returns the canonical spelling of a key press, or "" for a key that
// has none, such as Caps Lock.
func Name(k tea.Key) string {
	ctrl, alt := k.Mod.Contains(tea.ModCtrl), k.Mod.Contains(tea.ModAlt)
	shift := k.Mod.Contains(tea.ModShift)
	if !ctrl && !alt && k.Text != "" {
		// Text is what the key types, so "E" is Shift+e and "%" is Shift+5
		// without the user's keyboard layout mattering.
		if k.Text == " " {
			return "Space"
		}
		if utf8.RuneCountInString(k.Text) == 1 {
			return k.Text
		}
	}
	base, ok := names[k.Code]
	if !ok {
		if !unicode.IsPrint(k.Code) || unicode.IsSpace(k.Code) {
			return ""
		}
		base = string(unicode.ToLower(k.Code))
	}
	return canonical(ctrl, alt, shift, base)
}

// DisplayKey writes a canonical key for people: "C-b" becomes "Ctrl+b".
func DisplayKey(key string) string {
	var b strings.Builder
	for {
		matched := false
		for _, m := range mods {
			// A bare "-" or a single letter after the modifier is the key.
			if len(key) > len(m.letter) && key[:len(m.letter)] == m.letter {
				b.WriteString(m.display)
				key = key[len(m.letter):]
				matched = true
				break
			}
		}
		if !matched {
			break
		}
	}
	return b.String() + key
}
