package session

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/pkg/api"
)

// key handles one key press: the prefix, a command after the prefix, or a
// key for the pane.
func (m *Model) key(k tea.Key) tea.Cmd {
	// A prefix that types a character, such as `, stays text in a popup.
	if m.keys.IsPrefix(k) && !m.prefixed && k.Text == "" {
		if cmd, ok := m.closePopups(); ok {
			return tea.Batch(cmd, m.startPrefix(k))
		}
	}
	return m.dispatch(k)
}

// dispatch sends k to the open popup or mode, the prefix, or the pane.
func (m *Model) dispatch(k tea.Key) tea.Cmd {
	switch {
	case m.authPick != nil:
		return m.authKey(k)
	case m.over != nil:
		return m.overKey(k)
	case m.modelPick != nil:
		return m.modelKey(k)
	case m.picker != nil:
		m.pickerKey(k)
		return nil
	case m.switcher != nil:
		return m.switcherKey(k)
	case m.prompt != nil:
		return m.promptKey(k)
	case m.copy != nil:
		return m.copyKey(k)
	case m.confirm != nil:
		m.confirmKey(k)
		return nil
	case m.rename != nil:
		m.renameKey(k)
		return nil
	case m.menu != nil:
		return m.menuKey(k)
	case m.prefixed:
		m.endPrefix()
		if a, ok := m.keys.Lookup(k); ok {
			return m.run(a.Name, k)
		}
		return nil
	case m.keys.IsPrefix(k):
		return m.startPrefix(k)
	case m.focusedOwn() != nil:
		return m.ownKey(k)
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

// maxPopups bounds closePopups: one Escape closes a popup, and a popup can
// sit on another, such as the confirm question over the session list.
const maxPopups = 4

// closePopups closes every open popup the way Escape does, so each runs its
// own cleanup, and reports whether one was open and all are now closed,
// with the commands the Escapes returned. Copy mode is not a popup and
// stays. It calls dispatch, not key: a prefix bound to Escape would
// otherwise come back here without end.
func (m *Model) closePopups() (tea.Cmd, bool) {
	if !m.modal() || m.copy != nil {
		return nil, false
	}
	var cmds []tea.Cmd
	for range maxPopups {
		cmds = append(cmds, m.dispatch(tea.Key{Code: tea.KeyEscape}))
		if !m.modal() {
			return tea.Batch(cmds...), true
		}
	}
	return tea.Batch(cmds...), false
}
