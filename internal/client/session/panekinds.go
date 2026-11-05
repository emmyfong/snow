package session

import "github.com/emmyfong/snow/pkg/api"

// ownKinds makes the pane the client draws for each pane kind it knows.
// The server draws a terminal itself. Any other kind not listed here gets
// an unknownPane, so a newer server's pane never breaks this client.
var ownKinds = map[string]func(m *Model, id int) ownPane{
	api.PaneChat: func(m *Model, id int) ownPane { return newChatPane(id, m.theme, m.now, !m.hideUsage) },
}

// newOwnPane returns the pane for kind, or nil for a terminal pane.
func (m *Model) newOwnPane(kind string, id int) ownPane {
	if kind == api.PaneTerminal || kind == "" {
		return nil
	}
	if mk, ok := ownKinds[kind]; ok {
		return mk(m, id)
	}
	return newUnknownPane(kind, m.theme)
}
