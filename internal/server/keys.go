package server

import (
	"unicode/utf8"

	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/pkg/api"
)

// termKeys maps protocol key names to term key codes.
var termKeys = map[string]rune{
	api.KeyUp: term.KeyUp, api.KeyDown: term.KeyDown, api.KeyLeft: term.KeyLeft, api.KeyRight: term.KeyRight,
	api.KeyHome: term.KeyHome, api.KeyEnd: term.KeyEnd, api.KeyPgUp: term.KeyPgUp, api.KeyPgDown: term.KeyPgDown,
	api.KeyInsert: term.KeyInsert, api.KeyDelete: term.KeyDelete, api.KeyEnter: term.KeyEnter,
	api.KeyTab: term.KeyTab, api.KeyBackspace: term.KeyBackspace, api.KeyEscape: term.KeyEscape, api.KeySpace: ' ',
	api.KeyF1: term.KeyF1, api.KeyF2: term.KeyF2, api.KeyF3: term.KeyF3, api.KeyF4: term.KeyF4,
	api.KeyF5: term.KeyF5, api.KeyF6: term.KeyF6, api.KeyF7: term.KeyF7, api.KeyF8: term.KeyF8,
	api.KeyF9: term.KeyF9, api.KeyF10: term.KeyF10, api.KeyF11: term.KeyF11, api.KeyF12: term.KeyF12,
}

var termMods = map[string]term.Mod{
	api.ModShift: term.ModShift, api.ModAlt: term.ModAlt, api.ModCtrl: term.ModCtrl, api.ModMeta: term.ModMeta,
}

// toTermKey converts a validated protocol key for a pane's emulator.
func toTermKey(k api.Key) term.Key {
	code, ok := termKeys[k.Code]
	if !ok {
		code, _ = utf8.DecodeRuneInString(k.Code)
	}
	var mod term.Mod
	for _, m := range k.Mod {
		mod |= termMods[m]
	}
	return term.Key{Code: code, Mod: mod, Text: k.Text}
}
