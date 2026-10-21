package home

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/keys"
	"github.com/emmyfong/snow/pkg/api"
)

// naming is the second step of /rename: the new name for a picked session,
// typed in the input box.
type naming struct {
	session string // the session being renamed
	name    string // the new name typed so far
}

// startNaming asks for a new name for session, starting from its current
// name.
func (m *Model) startNaming(session string) {
	m.naming = &naming{session: session, name: session}
	m.input, m.cmdSel = "", 0
}

// namingKey edits the new name: Enter renames, Esc goes back to picking the
// session, and a refused name keeps the prompt open with the reason.
func (m *Model) namingKey(k tea.Key) tea.Cmd {
	n := m.naming
	switch k.Code {
	case tea.KeyEscape:
		m.naming = nil
		m.input, m.target = "/rename ", n.session
	case tea.KeyEnter:
		if err := api.ValidSessionName(n.name); err != nil {
			m.notice = "/rename: " + reason(err)
			return nil
		}
		m.naming = nil
		m.done()
		if n.session == m.back {
			m.back = n.name
		}
		return sendCmd(&api.RenameSession{Session: n.session, Name: n.name})
	default:
		if s, ok := keys.Edit(n.name, k); ok {
			n.name = s
		}
	}
	return nil
}

// namingPrompt is the input box's prompt while a name is typed.
func (m *Model) namingPrompt() string {
	return "New name for " + strings.TrimSpace(m.naming.session) + ": "
}
