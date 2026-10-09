package api

import (
	"slices"
	"unicode/utf8"
)

// Key is one key press. Code is a single character ("a", "é") or one of the
// Key* names; Text is what the key types, if anything; Mod lists modifiers by
// name. Names keep the protocol readable and independent of any emulator.
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

// Valid reports whether k has a known code and known modifiers.
func (k Key) Valid() bool {
	if utf8.RuneCountInString(k.Code) != 1 && !slices.Contains(NamedKeys, k.Code) {
		return false
	}
	for _, m := range k.Mod {
		if !slices.Contains(modNames, m) {
			return false
		}
	}
	return true
}
