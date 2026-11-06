package home

import (
	"github.com/emmyfong/snow/internal/client/command"
	"github.com/emmyfong/snow/internal/client/helppanel"
)

// helpKeys are the keys the help panel lists after the commands, with the
// configured prefix.
func (m *Model) helpKeys() []helppanel.Key {
	prefix := m.keys.PrefixLabel()
	return []helppanel.Key{
		{Keys: "Enter", Desc: "open or create a session"},
		{Keys: "/", Desc: "type a command; Tab completes"},
		{Keys: "/rename, /kill", Desc: "arrows or a click pick the session"},
		{Keys: prefix + " s", Desc: "switch to another session"},
		{Keys: prefix + " :", Desc: "run a command inside a session"},
		{Keys: prefix + " d", Desc: "detach and come here"},
	}
}

// helpPanel draws the commands and keys in a frame centered on the screen.
func (m *Model) helpPanel() (box string, x, y int) {
	return helppanel.Panel(m.theme, m.cols, m.rows, command.In(command.Home), m.helpKeys())
}
