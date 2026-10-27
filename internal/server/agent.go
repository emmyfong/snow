package server

import (
	"github.com/emmyfong/snow/pkg/api"
)

// agentState records what the agent in a terminal pane reports, from a
// Claude Code hook through snow hook, and shows it to the pane's session.
// A pane the server does not have, and a report older than the pane's
// last one, change nothing.
func (st *state) agentState(m *api.AgentState) {
	if m.Validate() != nil {
		return
	}
	m.At = notAfterNow(m.At)
	for _, sess := range st.sessions {
		_, p := sess.findPane(m.Pane)
		if p == nil {
			continue
		}
		if p.term == nil || m.At.Before(p.agentAt) {
			return
		}
		p.agentAt = m.At
		if p.agent == m.State {
			return
		}
		p.agent = m.State
		st.agentChanged(sess)
		return
	}
}

// agentChanged shows a changed agent state to the clients of sess.
func (st *state) agentChanged(sess *session) {
	st.sendLayout(sess)
	st.syncNeedsYou(sess)
}

// syncNeedsYou sends the session list to every client when the count of
// agents in sess that wait for the user changed since they last heard.
func (st *state) syncNeedsYou(sess *session) {
	if n := needsYou(sess); n != sess.needsYou {
		sess.needsYou = n
		st.sessionsChanged()
	}
}

// needsYou counts the agents in sess that wait for the user.
func needsYou(sess *session) int {
	n := 0
	for _, w := range sess.windows {
		for _, p := range w.panes {
			if p.agentNow() == api.AgentNeedsYou {
				n++
			}
		}
	}
	return n
}

// sendLayout sends the layout of sess to its clients.
func (st *state) sendLayout(sess *session) {
	for c := range st.clients {
		if c.session == sess.name {
			st.enqueue(c, st.layout(sess))
		}
	}
}

// agentNow is the state of the agent in p: what snow hook last reported
// for a terminal pane, and for a chat pane what its turn does.
func (p *pane) agentNow() string {
	if p.chat == nil {
		return p.agent
	}
	snap := &p.chat.snap
	switch {
	case snap.Pending != nil, snap.Question != nil, snap.State == api.ChatStateApproval:
		return api.AgentNeedsYou
	case snap.State == api.ChatStateStreaming, snap.State == api.ChatStateTool:
		return api.AgentWorking
	}
	return api.AgentIdle
}

// windowAgent is the most urgent agent state of every pane in w, including
// panes a zoom hides.
func windowAgent(w *window) string {
	out := ""
	for _, p := range w.panes {
		out = api.MoreUrgent(out, p.agentNow())
	}
	return out
}
