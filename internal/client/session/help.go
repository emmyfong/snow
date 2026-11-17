package session

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/command"
	"github.com/emmyfong/snow/internal/client/helppanel"
	"github.com/emmyfong/snow/internal/client/keys"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/internal/client/theme"
)

type sessionHelp struct{ keys *keys.Table }

func (h sessionHelp) View(th theme.Theme, cols, rows int) (string, int, int) {
	var entries []helppanel.Key
	for _, e := range h.keys.Entries() {
		entries = append(entries, helppanel.Key{Keys: h.keys.PrefixLabel() + " " + e.Keys, Desc: e.Desc})
	}
	return helppanel.Panel(th, cols, rows, command.Grouped(command.In(command.InSession)), entries)
}

func (m *Model) openSessionHelp() tea.Cmd {
	return m.showOverlay(&overlay{
		popup: sessionHelp{keys: m.keys},
		key:   func(tea.Key) picker.Result { return picker.Cancelled },
		click: func(theme.Theme, int, int, int, int) picker.Result { return picker.Cancelled },
	}, "the help panel")
}
