package session

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/agents"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/pkg/api"
)

// setAgents keeps the server's agent list and shows it on an open board
// at once.
func (m *Model) setAgents(l *api.AgentList) {
	m.agents = l.Agents
	if m.over != nil && m.over.agents != nil {
		m.over.agents(l.Agents)
	}
}

// openBoard shows the agents board of this session. Enter on an agent
// focuses its pane, in whatever window it is.
func (m *Model) openBoard() tea.Cmd {
	if m.layout == nil {
		return nil
	}
	b := agents.NewBoard(m.layout.Session, !m.hideUsage, m.now)
	b.SetAgents(m.agents)
	return m.showOverlay(&overlay{
		popup:  b,
		key:    b.Key,
		click:  b.Click,
		agents: b.SetAgents,
		done: func(r picker.Result) tea.Cmd {
			if pane, ok := b.Choice(); ok && r == picker.Accepted {
				m.send(&api.Command{Op: api.OpFocus, Target: pane})
			}
			return nil
		},
	}, "the agents board")
}
