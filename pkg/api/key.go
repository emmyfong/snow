package api

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Key is one key press. Names keep the protocol readable and independent of
// any emulator.
//
//   - Code is the key itself, without Shift: a single lower-case or
//     non-letter character ("a", "1", "é"), or one of the Key* names. Space
//     is KeySpace, never " ". Shift+a is Code "a" with ModShift.
//   - Text is what the key types, if anything ("A" for Shift+a). It may be
//     empty for keys that type nothing.
//   - Mod is a set of modifier names: order does not matter, and no name
//     repeats.
type Key struct {
	Code string   `json:"code"`
	Text string   `json:"text,omitempty"`
	Mod  []string `json:"mod,omitempty"`
}

// Named keys.
const (
	KeyUp        = "up"
	KeyDown      = "down"
	KeyLeft      = "left"
	KeyRight     = "right"
	KeyHome      = "home"
	KeyEnd       = "end"
	KeyPgUp      = "pgup"
	KeyPgDown    = "pgdown"
	KeyInsert    = "insert"
	KeyDelete    = "delete"
	KeyEnter     = "enter"
	KeyTab       = "tab"
	KeyBackspace = "backspace"
	KeyEscape    = "escape"
	KeySpace     = "space"
	KeyF1        = "f1"
	KeyF2        = "f2"
	KeyF3        = "f3"
	KeyF4        = "f4"
	KeyF5        = "f5"
	KeyF6        = "f6"
	KeyF7        = "f7"
	KeyF8        = "f8"
	KeyF9        = "f9"
	KeyF10       = "f10"
	KeyF11       = "f11"
	KeyF12       = "f12"
)

// Modifier names.
const (
	ModShift = "shift"
	ModAlt   = "alt"
	ModCtrl  = "ctrl"
	ModMeta  = "meta"
)

// NamedKeys lists every named key, for clients that map their own key types.
var NamedKeys = []string{
	KeyUp, KeyDown, KeyLeft, KeyRight, KeyHome, KeyEnd, KeyPgUp, KeyPgDown,
	KeyInsert, KeyDelete, KeyEnter, KeyTab, KeyBackspace, KeyEscape, KeySpace,
	KeyF1, KeyF2, KeyF3, KeyF4, KeyF5, KeyF6, KeyF7, KeyF8, KeyF9, KeyF10, KeyF11, KeyF12,
}

var modNames = []string{ModShift, ModAlt, ModCtrl, ModMeta}

// keyAliases are the common names ParseKey takes for a named key.
var keyAliases = map[string]string{
	"return": KeyEnter,
	"esc":    KeyEscape,
}

// ErrInvalidKey is returned by ParseKey for a name it cannot parse.
var ErrInvalidKey = errors.New("api: invalid key")

// ParseKey parses a key name for a tool call. A name is a named key from
// NamedKeys ("enter", "up"), or one character ("a", "1", "é"), with its
// modifiers before it, joined by "+" ("ctrl+c", "shift+tab", "alt+enter").
// An uppercase letter alone is Shift with that letter ("A" is "shift+a").
// With a modifier, its case does not matter: "ctrl+C" is "ctrl+c". The names
// "return" and "esc" also mean enter and escape.
func ParseKey(name string) (Key, error) {
	if name == "" {
		return Key{}, fmt.Errorf("%w: no key name", ErrInvalidKey)
	}
	parts := strings.Split(name, "+")
	if slices.Contains(parts, "") {
		return Key{}, fmt.Errorf("%w: %q has an empty part", ErrInvalidKey, name)
	}
	base := parts[len(parts)-1]
	modifiers := parts[:len(parts)-1]
	k := Key{}
	for _, m := range modifiers {
		m = strings.ToLower(m)
		if !slices.Contains(modNames, m) {
			return Key{}, fmt.Errorf("%w: %q is not a modifier; use shift, alt, ctrl, or meta", ErrInvalidKey, m)
		}
		if slices.Contains(k.Mod, m) {
			return Key{}, fmt.Errorf("%w: %q repeats %s", ErrInvalidKey, name, m)
		}
		k.Mod = append(k.Mod, m)
	}

	lower := strings.ToLower(base)
	if alias, ok := keyAliases[lower]; ok {
		lower = alias
	}
	if slices.Contains(NamedKeys, lower) {
		k.Code = lower
		if lower == KeySpace {
			k.Text = " "
		}
		return k, nil
	}
	r, size := utf8.DecodeRuneInString(base)
	if size == 0 || size != len(base) || unicode.IsSpace(r) || unicode.IsControl(r) {
		return Key{}, fmt.Errorf("%w: %q is not a key; use a named key such as \"enter\" or \"ctrl+c\", or one character", ErrInvalidKey, name)
	}
	// The protocol keeps Code as the key without Shift.
	shifted := unicode.IsUpper(r)
	if shifted && len(modifiers) == 0 {
		k.Mod = append(k.Mod, ModShift)
	}
	k.Code = string(unicode.ToLower(r))
	switch {
	case slices.Contains(k.Mod, ModCtrl), slices.Contains(k.Mod, ModMeta):
		// The emulator encodes the control key; text would bypass it.
	case slices.Contains(k.Mod, ModShift):
		if unicode.IsLetter(r) {
			k.Text = string(unicode.ToUpper(r))
		}
	default:
		k.Text = string(r)
	}
	if !k.Valid() {
		return Key{}, fmt.Errorf("%w: %q is not a key", ErrInvalidKey, name)
	}
	return k, nil
}

// Valid reports whether k follows the rules above.
func (k Key) Valid() bool {
	if !slices.Contains(NamedKeys, k.Code) {
		r, size := utf8.DecodeRuneInString(k.Code)
		if size == 0 || size != len(k.Code) || unicode.IsSpace(r) || unicode.IsUpper(r) {
			return false
		}
	}
	for i, m := range k.Mod {
		if !slices.Contains(modNames, m) || slices.Contains(k.Mod[:i], m) {
			return false
		}
	}
	return true
}
