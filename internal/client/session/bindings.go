package session

import (
	"strconv"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/pkg/api"
)

// prefix is the key that starts a Snow command, as in tmux. Rebinding comes
// with the config file (#54).
var prefix = tea.Key{Code: 'b', Mod: tea.ModCtrl}

func isPrefix(k tea.Key) bool { return k.Code == prefix.Code && k.Mod == prefix.Mod }

// arrows maps arrow keys to directions.
var arrows = map[rune]string{
	tea.KeyLeft: api.DirLeft, tea.KeyRight: api.DirRight, tea.KeyUp: api.DirUp, tea.KeyDown: api.DirDown,
}

// key handles one key press: the prefix, a command after the prefix, or a
// key for the pane.
func (m *Model) key(k tea.Key) tea.Cmd {
	if m.picker != nil {
		m.pickerKey(k)
		return nil
	}
	if m.confirm != nil {
		m.answer(k)
		return nil
	}
	if m.prefixed {
		m.prefixed = false
		if isPrefix(k) {
			m.sendKey(k) // prefix twice types the prefix key itself
			return nil
		}
		return m.command(k)
	}
	if isPrefix(k) {
		m.prefixed = true
		return nil
	}
	m.sendKey(k)
	return nil
}

func (m *Model) sendKey(k tea.Key) {
	if key, ok := toAPIKey(k); ok {
		m.input(api.Input{Key: &key})
	}
}

// command runs the binding for a key pressed after the prefix. The keys are
// tmux's defaults; an unbound key does nothing.
func (m *Model) command(k tea.Key) tea.Cmd {
	if k.Code == 'd' && k.Mod == 0 {
		return func() tea.Msg { return DetachMsg{} }
	}
	if dir, ok := arrows[k.Code]; ok {
		switch k.Mod {
		case 0:
			m.paneCommand(api.OpFocus, api.ArgDir, dir)
		case tea.ModCtrl:
			m.paneCommand(api.OpResize, api.ArgDir, dir, api.ArgCells, "1")
		case tea.ModAlt:
			m.paneCommand(api.OpResize, api.ArgDir, dir, api.ArgCells, "5")
		}
		return nil
	}
	switch k.Text {
	case "%":
		m.openPicker("Split right", api.Command{Op: api.OpSplit, Args: map[string]string{api.ArgDir: api.DirRight}})
	case `"`:
		m.openPicker("Split down", api.Command{Op: api.OpSplit, Args: map[string]string{api.ArgDir: api.DirDown}})
	case "c":
		m.openPicker("New window", api.Command{Op: api.OpNewWindow, Args: map[string]string{}})
	case "n":
		m.send(&api.Command{Op: api.OpNextWindow})
	case "p":
		m.send(&api.Command{Op: api.OpPrevWindow})
	case "x":
		m.confirmClosePane()
	case "&":
		m.confirmCloseWindow()
	case "z":
		m.paneCommand(api.OpZoom)
	case "E":
		m.paneCommand(api.OpEven)
	case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
		n, _ := strconv.Atoi(k.Text)
		m.send(&api.Command{Op: api.OpSelectWindow, Target: n})
	}
	return nil
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
