package term

import (
	"strings"
	"unicode"

	uv "github.com/charmbracelet/ultraviolet"
)

// Key is one key press. Code is a Unicode code point for ordinary keys, or
// one of the Key* constants. Text holds the characters the key produces, if
// any. The pane's emulator encodes the key for the program's current mode.
type Key struct {
	Code rune
	Mod  Mod
	Text string
}

// Mod is a set of modifier keys.
type Mod uint16

// Modifier keys. The values match ultraviolet's, so conversion is a cast.
const (
	ModShift Mod = Mod(uv.ModShift)
	ModAlt   Mod = Mod(uv.ModAlt)
	ModCtrl  Mod = Mod(uv.ModCtrl)
	ModMeta  Mod = Mod(uv.ModMeta)
)

// Special keys.
const (
	KeyUp        = uv.KeyUp
	KeyDown      = uv.KeyDown
	KeyLeft      = uv.KeyLeft
	KeyRight     = uv.KeyRight
	KeyHome      = uv.KeyHome
	KeyEnd       = uv.KeyEnd
	KeyPgUp      = uv.KeyPgUp
	KeyPgDown    = uv.KeyPgDown
	KeyInsert    = uv.KeyInsert
	KeyDelete    = uv.KeyDelete
	KeyEnter     = uv.KeyEnter
	KeyTab       = uv.KeyTab
	KeyBackspace = uv.KeyBackspace
	KeyEscape    = uv.KeyEscape
	KeyF1        = uv.KeyF1
	KeyF2        = uv.KeyF2
	KeyF3        = uv.KeyF3
	KeyF4        = uv.KeyF4
	KeyF5        = uv.KeyF5
	KeyF6        = uv.KeyF6
	KeyF7        = uv.KeyF7
	KeyF8        = uv.KeyF8
	KeyF9        = uv.KeyF9
	KeyF10       = uv.KeyF10
	KeyF11       = uv.KeyF11
	KeyF12       = uv.KeyF12
)

// event converts k for vt. It leaves out Text: vt never reads it, but it
// matches keys by comparing whole events, so a key with text, such as Up with
// text x, matched nothing and sent its code as a broken character.
func (k Key) event() uv.KeyPressEvent {
	return uv.KeyPressEvent{Code: k.Code, Mod: uv.KeyMod(k.Mod)}
}

// text returns the bytes a key types when only Shift, Alt, or both modify
// it. vt's SendKey writes a printable key only when it has no modifier, and
// writes its code, not its text, so Shift+a sent nothing and the kitty shape
// of A (code a, text A) sent a (#236). Keys with Ctrl or Meta, special keys,
// keys that type nothing, and keys whose text holds a control character are
// left to vt, so a malformed key cannot write escape sequences.
func (k Key) text() (string, bool) {
	if k.Mod&^(ModShift|ModAlt) != 0 || k.Code > unicode.MaxRune {
		return "", false
	}
	s := k.Text
	if strings.ContainsFunc(s, unicode.IsControl) {
		return "", false
	}
	if s == "" && k.Mod&ModShift != 0 && unicode.IsLower(k.Code) {
		// Terminals send Alt+Shift+a with no text.
		s = string(unicode.ToUpper(k.Code))
	}
	if s == "" {
		return "", false
	}
	if k.Mod&ModAlt != 0 {
		s = "\x1b" + s
	}
	return s, true
}
