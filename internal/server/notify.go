package server

import (
	"github.com/emmyfong/snow/internal/notify"
	"github.com/emmyfong/snow/pkg/api"
)

// notifyChanges shows a desktop notification for each agent that started
// to need the user, or that finished a turn since before. A client whose
// terminal has focus and shows the pane already tells the user, so that
// agent is skipped.
func (st *state) notifyChanges(before, after []api.AgentInfo) {
	n := st.srv.opts.Notify
	if n == nil {
		return
	}
	was := make(map[int]api.AgentInfo, len(before))
	for _, a := range before {
		was[a.Pane] = a
	}
	for _, a := range after {
		old, known := was[a.Pane]
		switch {
		case a.State == api.AgentNeedsYou && (!known || old.State != api.AgentNeedsYou):
			st.notifyAgent(n, notify.NeedsYou, a)
		case a.State == api.AgentIdle && known && old.State == api.AgentWorking:
			st.notifyAgent(n, notify.TurnEnd, a)
		}
	}
}

// notifyAgent sends one notification for a unless the user already looks
// at its pane.
func (st *state) notifyAgent(n *notify.Notifier, kind notify.Kind, a api.AgentInfo) {
	if st.watched(a) {
		return
	}
	n.Notify(notify.Notice{Kind: kind, Agent: agentWord(a.Kind), Session: a.Session})
}

// watched reports whether a client whose terminal has focus shows a's pane:
// the agent is in the shown window's focused pane, and that client is
// attached to the session and focused.
func (st *state) watched(a api.AgentInfo) bool {
	sess := st.sessions[a.Session]
	if sess == nil {
		return false
	}
	w := sess.shown()
	if w == nil || w.index != a.Window || w.focused != a.Pane {
		return false
	}
	for c := range st.clients {
		if c.session == a.Session && c.focused {
			return true
		}
	}
	return false
}

// agentWord names an agent kind in a notification. It holds no folder, no
// pane name, and no text from the agent. An unknown kind gets a neutral
// word, so a new agent never lies in a notification.
func agentWord(kind string) string {
	switch kind {
	case api.AgentKindChat:
		return "Snow chat"
	case api.AgentKindClaudeCode:
		return "Claude Code"
	}
	return "An agent"
}
