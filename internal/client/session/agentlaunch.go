package session

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/agentlaunch"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/pkg/api"
)

// openAgentLaunch opens the form that starts a Claude Code agent in its
// own worktree and window, from pane id, or from the focused pane when id
// is 0.
func (m *Model) openAgentLaunch(id int) tea.Cmd {
	from := m.focused()
	if id != 0 {
		from = m.paneInfo(id)
	}
	if from == nil {
		return nil
	}
	target := from.ID
	form := agentlaunch.New(from.Folder, m.profiles.Agents, m.profiles.Roles)
	return m.showOverlay(&overlay{
		popup: form,
		key:   form.Key,
		click: form.Click,
		paste: form.Paste,
		done: func(r picker.Result) tea.Cmd {
			if r == picker.Accepted {
				m.send(form.Command(target))
			}
			return nil
		},
	}, "the new agent form")
}

// paneInfo returns pane id of the layout, or nil.
func (m *Model) paneInfo(id int) *api.PaneInfo {
	if m.layout == nil {
		return nil
	}
	for wi := range m.layout.Windows {
		w := &m.layout.Windows[wi]
		for pi := range w.Panes {
			if w.Panes[pi].ID == id {
				return &w.Panes[pi]
			}
		}
	}
	return nil
}

// cmdAgents runs "agents" in the command list: "new" opens the new agent
// form, and nothing opens the agents board.
func (m *Model) cmdAgents(rest string) tea.Cmd {
	switch strings.TrimSpace(rest) {
	case "":
		m.prompt = nil
		return m.openBoard()
	case "new":
		m.prompt = nil
		return m.openAgentLaunch(0)
	}
	m.prompt.notice = "agents takes new, or nothing for the board."
	return nil
}
