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
	for _, p := range m.own {
		m.giveAgents(p)
	}
}

// agentsAware is an own pane that shows which agents work where, such as
// the git pane's worktrees.
type agentsAware interface {
	SetAgents(agents []api.AgentInfo)
}

// giveAgents gives p this session's agents, if p shows them.
func (m *Model) giveAgents(p ownPane) {
	a, ok := p.(agentsAware)
	if !ok || m.layout == nil {
		return
	}
	var mine []api.AgentInfo
	for _, ag := range m.agents {
		if ag.Session == m.layout.Session {
			mine = append(mine, ag)
		}
	}
	a.SetAgents(mine)
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
