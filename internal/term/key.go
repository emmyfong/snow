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
	mod := k.Mod
	// vt has no bytes for Shift+Enter or Shift+Backspace, with or without
	// Alt, and sent nothing or a lone ESC (#240). vt has no kitty keyboard
	// mode to tell the program about Shift, so Shift+Enter goes out as
	// Alt+Enter, which Claude Code and many other programs read as a
	// newline, and Shift+Backspace as Backspace. Shift+Tab keeps its own
	// sequence.
	if mod&ModShift != 0 && mod&^(ModShift|ModAlt) == 0 {
		switch k.Code {
		case KeyEnter:
			mod = ModAlt
		case KeyBackspace:
			mod &^= ModShift
		}
	}
	return uv.KeyPressEvent{Code: k.Code, Mod: uv.KeyMod(mod)}
}

// text returns the bytes a key types when only Shift, Alt, or both modify
// it. vt's SendKey writes a printable key only when it has no modifier, and
// writes its code, not its text, so Shift+a sent nothing and the kitty shape
// of A (code a, text A) sent a (#236). Keys with Ctrl or Meta, special keys,
// keys with neither text nor a printable code, and keys whose text holds a
// control character are left to vt, so a malformed key cannot write escape
// sequences.
func (k Key) text() (string, bool) {
	if k.Mod&^(ModShift|ModAlt) != 0 || k.Code > unicode.MaxRune {
		return "", false
	}
	s := k.Text
	if strings.ContainsFunc(s, unicode.IsControl) {
		return "", false
	}
	if s == "" && k.Mod&ModShift != 0 && unicode.IsPrint(k.Code) {
		// Terminals send Alt+Shift+a, Alt+Shift+1, and Shift+Space with no
		// text. A letter's shifted form is known; a digit's or a symbol's
		// depends on the keyboard layout, so it is sent unshifted (#240).
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
