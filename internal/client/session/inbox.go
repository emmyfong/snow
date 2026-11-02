package session

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/agents"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/pkg/api"
)

// openInbox shows the needs-you inbox: every agent of every session that
// waits for the user, oldest first. Enter jumps to the agent's pane, in
// this session or another one.
func (m *Model) openInbox() tea.Cmd {
	if m.layout == nil {
		return nil
	}
	in := agents.NewInbox(m.now)
	in.SetAgents(m.agents)
	session := m.layout.Session
	return m.showOverlay(&overlay{
		popup:  in,
		key:    in.Key,
		click:  in.Click,
		agents: in.SetAgents,
		done: func(r picker.Result) tea.Cmd {
			a, ok := in.Choice()
			if !ok || r != picker.Accepted {
				return nil
			}
			if a.Session == session {
				m.send(&api.Command{Op: api.OpFocus, Target: a.Pane})
				return nil
			}
			return switchTo(SwitchMsg{Session: a.Session, Pane: a.Pane})
		},
	}, "the needs-you inbox")
}
