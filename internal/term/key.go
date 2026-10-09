package term

import uv "github.com/charmbracelet/ultraviolet"

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
	KeyF12       = uv.KeyF12
)

func (k Key) event() uv.KeyPressEvent {
	return uv.KeyPressEvent{Code: k.Code, Mod: uv.KeyMod(k.Mod), Text: k.Text}
}
