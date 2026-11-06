// Package runtimepick provides Snow-owned Claude connection choices. It reuses
// the shared list popup and masked key editor, with equal keyboard/mouse actions.
package runtimepick

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/authpick"
	"github.com/emmyfong/snow/internal/client/listpick"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// Action is a deliberate connection operation. Key is hidden in all formatting.
type Action struct{ Kind, Connection, Auth, Key string }

// Format prevents a pasted API key from entering debug output.
func (a Action) Format(f fmt.State, _ rune) {
	_, _ = fmt.Fprintf(f, "runtimepick.Action{Kind:%q, Connection:%q, Key:[hidden]}", a.Kind, a.Connection)
}

type choice struct {
	label, hint string
	action      Action
}

// Model owns one connection popup. It performs no I/O and never starts work
// after sign-in. The session sends only the action the user selected.
type Model struct {
	list        *listpick.Model
	key         *authpick.Model
	connections []api.RuntimeConnection
	choices     []choice
	selected    string
	actions     bool
	chosen      Action
	pending     string
}

// New opens a connection list while its metadata loads.
func New() *Model {
	m := &Model{list: listpick.New("Claude Code connections", []ui.Hint{{Key: "Enter", Desc: "choose"}, {Key: "Esc", Desc: "cancel"}})}
	m.list.SetNote("Loading Snow connections...")
	return m
}

// SetList replaces metadata without importing any standalone CLI account.
func (m *Model) SetList(response *api.RuntimeConnections) {
	m.connections = append([]api.RuntimeConnection(nil), response.Connections...)
	if response.Selected != "" {
		m.selected = response.Selected
	}
	if response.State == "error" {
		if m.key != nil {
			m.key.Fail(response.Message)
			return
		}
		m.list.SetTitle(response.Message)
	} else if m.pending == api.RuntimeSaveKey {
		m.key = nil
	}
	if m.pending == api.RuntimeCreate {
		m.actions = true
	}
	m.pending = ""
	m.rebuild()
}

func (m *Model) connection() (api.RuntimeConnection, bool) {
	for _, connection := range m.connections {
		if connection.ID == m.selected {
			return connection, true
		}
	}
	return api.RuntimeConnection{}, false
}

func (m *Model) rebuild() {
	m.choices = nil
	if connection, ok := m.connection(); m.actions && ok {
		m.list.SetTitle("Claude Code: " + connection.Environment)
		auth, login := "Subscription", "Sign in with subscription"
		kind := api.RuntimeLogin
		if connection.Auth == "api-key" {
			auth, login, kind = "API billing", "Enter an API key", "key"
		}
		m.choices = append(m.choices,
			choice{label: "Use for this chat", hint: auth + "; no task starts", action: Action{Kind: api.RuntimeUse, Connection: connection.ID}},
			choice{label: login, hint: "Separate Snow account", action: Action{Kind: kind, Connection: connection.ID}},
			choice{label: "Check connection", hint: connection.Status, action: Action{Kind: api.RuntimeStatus, Connection: connection.ID}},
			choice{label: "Sign out of this connection", hint: "Keep saved conversations", action: Action{Kind: api.RuntimeLogout, Connection: connection.ID}},
			choice{label: "Back to connections", action: Action{Kind: "back"}})
	} else {
		m.actions = false
		m.list.SetTitle("Claude Code connections")
		for _, connection := range m.connections {
			auth := "Subscription"
			if connection.Auth == "api-key" {
				auth = "API billing"
			}
			id := connection.ID[:min(6, len(connection.ID))]
			m.choices = append(m.choices, choice{label: connection.Name + " (" + id + ")", hint: auth + ", " + connection.Environment,
				action: Action{Kind: "open", Connection: connection.ID}})
		}
		m.choices = append(m.choices,
			choice{label: "Add a subscription connection", hint: "Sign in through Snow", action: Action{Kind: api.RuntimeCreate, Auth: "subscription"}},
			choice{label: "Add an API-key connection", hint: "Separate API billing", action: Action{Kind: api.RuntimeCreate, Auth: "api-key"}})
	}
	var rows [][]string
	for _, choice := range m.choices {
		rows = append(rows, []string{choice.label, choice.hint})
	}
	m.list.SetRows(rows)
}

// Action returns the operation selected by the most recent Accepted result.
func (m *Model) Action() Action {
	action := m.chosen
	m.chosen.Key = ""
	return action
}

// Key handles a keyboard action; internal navigation keeps the popup open.
func (m *Model) Key(key tea.Key) picker.Result {
	if m.key != nil {
		if key.Code == tea.KeyEscape {
			m.key = nil
			return picker.Open
		}
		return m.keyResult(m.key.Key(key))
	}
	return m.result(m.list.Key(key))
}

// Paste adds text only to an open masked key field.
func (m *Model) Paste(text string) {
	if m.key != nil {
		m.key.Paste(text)
	}
}

// Click uses the same choices as the keyboard.
func (m *Model) Click(th theme.Theme, cols, rows, x, y int) picker.Result {
	if m.key != nil {
		return m.keyResult(m.key.Click(th, cols, rows, x, y))
	}
	return m.result(m.list.Click(th, cols, rows, x, y))
}

func (m *Model) keyResult(result authpick.Result) picker.Result {
	if result == authpick.Cancelled {
		m.key = nil
		return picker.Open
	}
	if result != authpick.Submit {
		return picker.Open
	}
	m.chosen = Action{Kind: api.RuntimeSaveKey, Connection: m.selected, Key: m.key.KeyValue()}
	m.pending = m.chosen.Kind
	return picker.Accepted
}

func (m *Model) result(result picker.Result) picker.Result {
	if result != picker.Accepted {
		return result
	}
	index := m.list.Selected()
	if index < 0 || index >= len(m.choices) {
		return picker.Open
	}
	action := m.choices[index].action
	switch action.Kind {
	case "open":
		m.selected, m.actions = action.Connection, true
		m.rebuild()
		m.list.Select(0)
		return picker.Open
	case "back":
		m.actions = false
		m.rebuild()
		return picker.Open
	case "key":
		m.key = authpick.New()
		m.key.EnterKey(api.ProviderAuth{Name: "claude-code", Title: "Claude Code API key", Runtime: "claude-code"})
		return picker.Open
	}
	m.chosen, m.pending = action, action.Kind
	return picker.Accepted
}

// View draws the shared popup or masked editor at the current terminal size.
func (m *Model) View(th theme.Theme, cols, rows int) (string, int, int) {
	if m.key != nil {
		return m.key.View(th, cols, rows)
	}
	return m.list.View(th, cols, rows)
}
