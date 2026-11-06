// Package runtimepick provides Snow-owned Claude connection choices. It reuses
// the shared list popup and masked key editor, with equal keyboard/mouse actions.
package runtimepick

import (
	"fmt"
	"strings"

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
	removing    bool // asking whether to remove the selected connection
	adding      bool // the key field is for a new API key connection
	chosen      Action
	pending     string
	ignored     []string
	failure     string // the last refused operation, shown above the choices
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
	m.ignored = response.Ignored
	if response.Selected != "" {
		m.selected = response.Selected
	}
	m.failure = ""
	if response.State == "error" {
		if m.key != nil {
			m.key.Fail(response.Message)
			return
		}
		m.failure = response.Message
	} else if m.pending == api.RuntimeSaveKey || m.pending == api.RuntimeCreate {
		m.key, m.adding = nil, false
	}
	m.removing = false
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
	if connection, ok := m.connection(); m.actions && ok && m.removing {
		m.rebuildRemove(connection)
	} else if m.actions && ok {
		m.rebuildActions(connection)
	} else {
		m.rebuildList()
	}
	var rows [][]string
	for _, choice := range m.choices {
		rows = append(rows, []string{choice.label, choice.hint})
	}
	m.list.SetRows(rows)
}

// rebuildList shows every connection with its state and billing, then the
// two ways to add one.
func (m *Model) rebuildList() {
	m.actions = false
	m.list.SetTitle("Claude Code connections")
	detail := "Each connection has its own Claude Code sign-in, separate from the claude command."
	if len(m.connections) == 0 {
		detail = "Snow keeps its own Claude Code sign-in, separate from the claude command. Choose how Claude Code is paid for."
	}
	if len(m.ignored) > 0 {
		detail += " Snow ignores " + strings.Join(m.ignored, ", ") + " from this machine."
	}
	m.list.SetDetail(m.withFailure(detail))
	for _, connection := range m.connections {
		short := "Claude plan"
		if connection.Auth == "api-key" {
			short = "API billing"
		}
		m.choices = append(m.choices, choice{label: name(connection) + "  " + statusText(connection),
			hint: short + ", " + connection.Environment, action: Action{Kind: "open", Connection: connection.ID}})
	}
	m.choices = append(m.choices,
		choice{label: "Add: sign in with a Claude plan", hint: billing("subscription"), action: Action{Kind: api.RuntimeCreate, Auth: "subscription"}},
		choice{label: "Add: use an API key", hint: billing("api-key"), action: Action{Kind: "add-key"}})
}

// rebuildActions shows what the user can do with one connection. A
// connection the policy blocks offers no way to use or sign in to it.
func (m *Model) rebuildActions(connection api.RuntimeConnection) {
	m.list.SetTitle(name(connection) + ", " + connection.Environment)
	detail := billing(connection.Auth) + ". " + statusText(connection) + "."
	if connection.Reason != "" {
		detail += " " + connection.Reason
	}
	m.list.SetDetail(m.withFailure(detail))
	id := connection.ID
	if connection.Status != api.RuntimeStatusBlocked {
		m.choices = append(m.choices, choice{label: "Use for this chat", hint: "No task starts", action: Action{Kind: api.RuntimeUse, Connection: id}})
		if connection.Auth == "api-key" {
			m.choices = append(m.choices, choice{label: "Enter an API key", hint: "Saved for this connection only", action: Action{Kind: "key", Connection: id}})
		} else {
			m.choices = append(m.choices, choice{label: "Sign in", hint: "Opens Claude's sign-in in a Snow window", action: Action{Kind: api.RuntimeLogin, Connection: id}})
		}
	}
	signOut := "Keeps saved chats"
	if connection.Auth == "api-key" {
		signOut = "Removes the saved key; keeps saved chats"
	}
	m.choices = append(m.choices,
		choice{label: "Check status", hint: "No model request", action: Action{Kind: api.RuntimeStatus, Connection: id}},
		choice{label: "Sign out", hint: signOut, action: Action{Kind: api.RuntimeLogout, Connection: id}},
		choice{label: "Remove connection", hint: "Asks first; keeps saved chats", action: Action{Kind: "ask-remove"}},
		choice{label: "Back to connections", action: Action{Kind: "back"}})
}

// rebuildRemove asks before a connection is removed, and says what goes
// and what stays.
func (m *Model) rebuildRemove(connection api.RuntimeConnection) {
	m.list.SetTitle("Remove " + name(connection) + "?")
	gone := "Snow signs this connection out and deletes its sign-in."
	if connection.Auth == "api-key" {
		gone = "Snow deletes this connection and its saved API key."
	}
	m.list.SetDetail(m.withFailure(gone + " Your saved chats stay."))
	m.choices = append(m.choices,
		choice{label: "Remove", hint: "Cannot be undone", action: Action{Kind: api.RuntimeRemove, Connection: connection.ID}},
		choice{label: "Keep it", action: Action{Kind: "keep"}})
}

// withFailure puts the last failure before detail, so the connection's own
// reason stays visible after a refused operation.
func (m *Model) withFailure(detail string) string {
	if m.failure == "" {
		return detail
	}
	return m.failure + " " + detail
}

func name(c api.RuntimeConnection) string {
	return c.Name + " (" + c.ID[:min(6, len(c.ID))] + ")"
}

// billing says in plain words who pays for a route.
func billing(auth string) string {
	if auth == "api-key" {
		return "Billed to your API account"
	}
	return "Uses your Claude plan"
}

// statusText says a connection's state. Signed in and ready are separate:
// ready needs a runtime that started. Unknown never reads as signed out.
func statusText(c api.RuntimeConnection) string {
	switch c.Status {
	case api.RuntimeStatusSignedIn:
		if c.Ready {
			return "Signed in, ready"
		}
		return "Signed in"
	case api.RuntimeStatusKeySaved:
		if c.Ready {
			return "Key saved, ready"
		}
		return "Key saved"
	case api.RuntimeStatusSignedOut:
		if c.Auth == "api-key" {
			return "No key saved"
		}
		return "Signed out"
	case api.RuntimeStatusBlocked:
		return "Blocked by policy"
	}
	if c.Reason != "" {
		return "Status unknown"
	}
	return "Not checked yet"
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
			m.key, m.adding = nil, false
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
		m.key, m.adding = nil, false
		return picker.Open
	}
	if result != authpick.Submit {
		return picker.Open
	}
	m.chosen = Action{Kind: api.RuntimeSaveKey, Connection: m.selected, Key: m.key.KeyValue()}
	if m.adding {
		// Snow saves a new API key connection only together with its key.
		m.chosen = Action{Kind: api.RuntimeCreate, Auth: "api-key", Key: m.key.KeyValue()}
	}
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
		// Checking runs Claude's status command, which sends no model request.
		m.chosen = Action{Kind: api.RuntimeStatus, Connection: action.Connection}
		m.pending = m.chosen.Kind
		return picker.Accepted
	case "back":
		m.actions = false
		m.rebuild()
		return picker.Open
	case "key", "add-key":
		m.key, m.adding = authpick.New(), action.Kind == "add-key"
		m.key.EnterKey(api.ProviderAuth{Name: "claude-code", Title: "Claude Code API key", Runtime: "claude-code"})
		return picker.Open
	case "ask-remove", "keep":
		m.removing = action.Kind == "ask-remove"
		m.rebuild()
		// The question starts on "Keep it", so a stray Enter removes nothing.
		m.list.Select(map[bool]int{true: 1, false: 0}[m.removing])
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
