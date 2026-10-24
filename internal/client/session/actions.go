package session

import (
	"strconv"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/keys"
	"github.com/emmyfong/snow/pkg/api"
)

// handler runs one action. k is the key that triggered it, or the zero key
// when the mouse did. Send-prefix does not use it: it sends the prefix key,
// which may differ from the key that ran it.
type handler func(m *Model, k tea.Key) tea.Cmd

// handlers maps each action name in internal/client/keys to what it does on
// this screen.
var handlers = buildHandlers()

var dirArgs = map[string]string{
	"left": api.DirLeft, "right": api.DirRight, "up": api.DirUp, "down": api.DirDown,
}

func buildHandlers() map[string]handler {
	h := map[string]handler{
		"split-right": func(m *Model, _ tea.Key) tea.Cmd {
			m.openPicker("Split right", api.Command{Op: api.OpSplit, Args: map[string]string{api.ArgDir: api.DirRight}})
			return nil
		},
		"split-down": func(m *Model, _ tea.Key) tea.Cmd {
			m.openPicker("Split down", api.Command{Op: api.OpSplit, Args: map[string]string{api.ArgDir: api.DirDown}})
			return nil
		},
		"new-window": func(m *Model, _ tea.Key) tea.Cmd {
			m.openPicker("New window", api.Command{Op: api.OpNewWindow, Args: map[string]string{}})
			return nil
		},
		"next-window":    func(m *Model, _ tea.Key) tea.Cmd { m.send(&api.Command{Op: api.OpNextWindow}); return nil },
		"prev-window":    func(m *Model, _ tea.Key) tea.Cmd { m.send(&api.Command{Op: api.OpPrevWindow}); return nil },
		"rename-window":  func(m *Model, _ tea.Key) tea.Cmd { m.startRename(); return nil },
		"close-pane":     func(m *Model, _ tea.Key) tea.Cmd { m.confirmClosePane(); return nil },
		"close-window":   func(m *Model, _ tea.Key) tea.Cmd { m.confirmCloseWindow(); return nil },
		"zoom":           func(m *Model, _ tea.Key) tea.Cmd { m.paneCommand(api.OpZoom); return nil },
		"even-out":       func(m *Model, _ tea.Key) tea.Cmd { m.paneCommand(api.OpEven); return nil },
		"detach":         func(*Model, tea.Key) tea.Cmd { return func() tea.Msg { return DetachMsg{} } },
		"sessions":       func(m *Model, _ tea.Key) tea.Cmd { m.openSessions(); return nil },
		"command-prompt": func(m *Model, _ tea.Key) tea.Cmd { m.openPrompt(); return nil },
		"copy-mode":      func(m *Model, _ tea.Key) tea.Cmd { m.enterCopy(); return nil },
		keys.SendPrefix:  func(m *Model, _ tea.Key) tea.Cmd { m.sendKey(m.prefixKey); return nil },
	}
	for i := range 10 {
		h["select-window-"+strconv.Itoa(i)] = func(m *Model, _ tea.Key) tea.Cmd {
			m.send(&api.Command{Op: api.OpSelectWindow, Target: i})
			return nil
		}
	}
	for name, dir := range dirArgs {
		h["focus-"+name] = func(m *Model, _ tea.Key) tea.Cmd {
			m.paneCommand(api.OpFocus, api.ArgDir, dir)
			return nil
		}
		for suffix, cells := range map[string]string{"": "1", "-5": "5"} {
			h["resize-"+name+suffix] = func(m *Model, _ tea.Key) tea.Cmd {
				m.paneCommand(api.OpResize, api.ArgDir, dir, api.ArgCells, cells)
				return nil
			}
		}
	}
	return h
}

// run does the action called name. It is how a key and, later, the mouse
// reach the same behavior.
func (m *Model) run(name string, k tea.Key) tea.Cmd {
	if h := handlers[name]; h != nil {
		return h(m, k)
	}
	return nil
}
