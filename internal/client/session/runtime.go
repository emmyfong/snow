package session

import (
	"strconv"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/internal/client/runtimepick"
	"github.com/emmyfong/snow/pkg/api"
)

// runtimeClose is the step after a new connection's use: close the picker.
const runtimeClose = "close"

type runtimeChooser interface {
	RuntimeInfo() *api.RuntimeInfo
	ConnectionRequest(action, connection, auth, key string) *api.RuntimeRequest
	RuntimeInput(kind, id, run, text string) *api.ChatInput
}

func (m *Model) openRuntimePicker(pane int) tea.Cmd {
	if _, ok := m.own[pane].(runtimeChooser); !ok {
		return m.ShowError("Coding-agent connections work in chat panes.")
	}
	m.firstRun[pane] = true
	m.runtimePick, m.runtimePane, m.runtimeAfter = runtimepick.New(), pane, ""
	if command := m.showRuntimePicker(); command != nil {
		return command
	}
	m.sendRuntimeAction(runtimepick.Action{Kind: api.RuntimeList})
	return nil
}

func (m *Model) showRuntimePicker() tea.Cmd {
	pick := m.runtimePick
	return m.showOverlay(&overlay{pane: m.runtimePane, popup: pick, key: pick.Key, click: pick.Click, paste: pick.Paste,
		done: func(result picker.Result) tea.Cmd {
			if result == picker.Cancelled {
				m.runtimeAfter, m.runtimeRequestID = "", ""
				m.runtimePick = nil
				return nil
			}
			action := pick.Action()
			if action.Kind == api.RuntimeCreate {
				// A connection exists only once it works, so the chat
				// switches to it as soon as it is added.
				m.runtimeAfter = api.RuntimeUse
			}
			if action.Kind == api.RuntimeLogin {
				chooser := m.own[m.runtimePane].(runtimeChooser)
				if chooser.RuntimeInfo() == nil || chooser.RuntimeInfo().Connection != action.Connection {
					m.runtimeAfter = api.RuntimeLogin
					action.Kind = api.RuntimeUse
				}
			}
			m.sendRuntimeAction(action)
			return m.showRuntimePicker()
		}}, "Claude Code connections")
}

func (m *Model) sendRuntimeAction(action runtimepick.Action) {
	chooser, ok := m.own[m.runtimePane].(runtimeChooser)
	if !ok {
		return
	}
	m.runtimeSeq++
	m.runtimeRequestID = strconv.FormatUint(m.runtimeSeq, 10)
	request := chooser.ConnectionRequest(action.Kind, action.Connection, action.Auth, action.Key)
	request.RequestID = m.runtimeRequestID
	m.send(request)
}

func (m *Model) runtimeConnections(response *api.RuntimeConnections) tea.Cmd {
	if response.Pane != m.runtimePane || response.RequestID != m.runtimeRequestID {
		return nil
	}
	chooser, ok := m.own[response.Pane].(runtimeChooser)
	if !ok {
		return nil
	}
	if chooser.ConnectionRequest(api.RuntimeList, "", "", "").Agent != response.Agent {
		return nil
	}
	if response.State == "signing-in" {
		m.runtimePick, m.over = nil, nil
		return m.showNotice(response.Message)
	}
	if m.runtimePick != nil {
		m.runtimePick.SetList(response)
	}
	if response.State == "error" {
		m.runtimeAfter = ""
		return m.ShowError(response.Message)
	}
	switch m.runtimeAfter {
	case api.RuntimeLogin:
		m.runtimeAfter = ""
		m.sendRuntimeAction(runtimepick.Action{Kind: api.RuntimeLogin, Connection: response.Selected})
		return nil
	case api.RuntimeUse:
		m.runtimeAfter = ""
		// A failed sign-in added nothing, and its notice says so.
		if response.Selected != "" {
			m.runtimeAfter = runtimeClose
			m.sendRuntimeAction(runtimepick.Action{Kind: api.RuntimeUse, Connection: response.Selected})
		}
		if response.Message != "" {
			return m.showNotice(response.Message)
		}
		return nil
	case runtimeClose:
		m.runtimeAfter = ""
		if m.runtimePick != nil {
			m.runtimePick, m.over = nil, nil
		}
	}
	if runtime := chooser.RuntimeInfo(); runtime != nil && runtime.Connection == response.Selected && m.runtimePick != nil && m.runtimePick.Action().Kind == api.RuntimeUse {
		m.runtimePick, m.over = nil, nil
	}
	if response.Message != "" {
		return m.showNotice(response.Message)
	}
	return nil
}
