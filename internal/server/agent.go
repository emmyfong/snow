package server

import (
	"time"

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
		if p.term == nil || p.ended {
			return
		}
		p.sawHook = true
		if p.hookless {
			p.hookless = false
			st.syncHint()
		}
		if m.At.Before(p.agentAt) {
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

// defaultHookWait is how long an agent pane may take to send its first
// hook. Claude Code sends SessionStart as it starts, but a first start can
// wait on its trust question, or on a slow WSL distribution.
const defaultHookWait = 30 * time.Second

// hooksNotSet is the home page's hint while an agent pane sends no hooks.
const hooksNotSet = "Claude Code sends Snow no hooks: run snow claude setup"

// hooksDue marks pane id as sending no hooks when none came in time.
func (st *state) hooksDue(id int) {
	for _, sess := range st.sessions {
		if _, p := sess.findPane(id); p != nil {
			if !p.sawHook {
				p.hookless = true
				st.syncHint()
			}
			return
		}
	}
}

// hookHint is the home page's hint now: hooksNotSet while an agent pane
// that sent no hook in time is open.
func (st *state) hookHint() string {
	for _, sess := range st.sessions {
		for _, w := range sess.windows {
			for _, p := range w.panes {
				if p.hookless {
					return hooksNotSet
				}
			}
		}
	}
	return ""
}

// syncHint sends the session list when the hint changed since the clients
// last heard.
func (st *state) syncHint() {
	if st.hookHint() != st.hint {
		st.sessionsChanged()
	}
}

// userTyped shows the agent in p working when the user types into it
// while it waits. Hooks cannot see the user answer a permission question,
// allow a long command, or press Esc; the next hook corrects the guess.
func (st *state) userTyped(sess *session, p *pane) {
	if p.term == nil || p.agent != api.AgentNeedsYou {
		return
	}
	p.agent, p.agentAt = api.AgentWorking, time.Now()
	st.agentChanged(sess)
}

// agentGone forgets the agent of p, whose program is gone, such as a pane
// whose WSL distribution stopped: no hook will clear it.
func (st *state) agentGone(sess *session, p *pane) {
	hinted := p.hookless
	p.hookless = false
	if p.agent != "" {
		p.agent = ""
		st.agentChanged(sess)
	}
	if hinted {
		st.syncHint()
	}
}
