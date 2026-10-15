package session

import (
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/pkg/api"
)

// apiKeys maps Bubble Tea's special key codes to protocol key names.
var apiKeys = map[rune]string{
	tea.KeyUp: api.KeyUp, tea.KeyDown: api.KeyDown, tea.KeyLeft: api.KeyLeft, tea.KeyRight: api.KeyRight,
	tea.KeyHome: api.KeyHome, tea.KeyEnd: api.KeyEnd, tea.KeyPgUp: api.KeyPgUp, tea.KeyPgDown: api.KeyPgDown,
	tea.KeyInsert: api.KeyInsert, tea.KeyDelete: api.KeyDelete, tea.KeyEnter: api.KeyEnter,
	tea.KeyTab: api.KeyTab, tea.KeyBackspace: api.KeyBackspace, tea.KeyEscape: api.KeyEscape, tea.KeySpace: api.KeySpace,
	tea.KeyF1: api.KeyF1, tea.KeyF2: api.KeyF2, tea.KeyF3: api.KeyF3, tea.KeyF4: api.KeyF4,
	tea.KeyF5: api.KeyF5, tea.KeyF6: api.KeyF6, tea.KeyF7: api.KeyF7, tea.KeyF8: api.KeyF8,
	tea.KeyF9: api.KeyF9, tea.KeyF10: api.KeyF10, tea.KeyF11: api.KeyF11, tea.KeyF12: api.KeyF12,
}

var apiMods = []struct {
	bit  tea.KeyMod
	name string
}{{tea.ModShift, api.ModShift}, {tea.ModAlt, api.ModAlt}, {tea.ModCtrl, api.ModCtrl}, {tea.ModMeta, api.ModMeta}}

// toAPIKey converts a key press for the protocol. It reports false for keys
// the protocol has no name for.
func toAPIKey(k tea.Key) (api.Key, bool) {
	code, ok := apiKeys[k.Code]
	if !ok {
		if !unicode.IsPrint(k.Code) || unicode.IsSpace(k.Code) {
			return api.Key{}, false
		}
		code = string(unicode.ToLower(k.Code))
	}
	out := api.Key{Code: code, Text: k.Text}
	for _, m := range apiMods {
		if k.Mod&m.bit != 0 {
			out.Mod = append(out.Mod, m.name)
		}
	}
	return out, true
}
