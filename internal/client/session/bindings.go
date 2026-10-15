package session

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/pkg/api"
)

// key handles one key press: the prefix, a command after the prefix, or a
// key for the pane.
func (m *Model) key(k tea.Key) tea.Cmd {
	switch {
	case m.picker != nil:
		m.pickerKey(k)
		return nil
	case m.confirm != nil:
		m.answer(k)
		return nil
	case m.rename != nil:
		m.renameKey(k)
		return nil
	case m.prefixed:
		m.endPrefix()
		if a, ok := m.keys.Lookup(k); ok {
			return m.run(a.Name, k)
		}
		return nil
	case m.keys.IsPrefix(k):
		return m.startPrefix()
	}
	m.sendKey(k)
	return nil
}

func (m *Model) sendKey(k tea.Key) {
	if key, ok := toAPIKey(k); ok {
		m.input(api.Input{Key: &key})
	}
}

// paneCommand sends op for the focused pane with args given as key, value
// pairs. It does nothing before a pane is known.
func (m *Model) paneCommand(op string, args ...string) {
	p := m.focused()
	if p == nil {
		return
	}
	cmd := &api.Command{Op: op, Target: p.ID}
	if len(args) > 0 {
		cmd.Args = map[string]string{}
		for i := 0; i+1 < len(args); i += 2 {
			cmd.Args[args[i]] = args[i+1]
		}
	}
	m.send(cmd)
}
