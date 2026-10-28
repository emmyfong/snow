package server

import (
	"maps"
	"slices"
	"time"

	"github.com/emmyfong/snow/internal/notify"
	"github.com/emmyfong/snow/pkg/api"
)

// agentKind is the kind of agent p runs, or "" when it runs none: a chat
// pane, or a terminal pane that runs an agent profile or whose program
// reports through snow hook, such as claude started by hand in a shell.
func (st *state) agentKind(p *pane) string {
	switch {
	case p.chat != nil:
		return api.AgentKindChat
	case p.agent != "" || st.srv.launch.isAgent(p.profile):
		return api.AgentKindClaudeCode
	}
	return ""
}

// agents lists every agent in the running sessions, by session name,
// window index, and pane ID. It stamps when each agent entered its state,
// so it runs after every change that can move one (see syncAgents).
func (st *state) agents() []api.AgentInfo {
	now := time.Now()
	out := []api.AgentInfo{}
	for _, name := range slices.Sorted(maps.Keys(st.sessions)) {
		sess := st.sessions[name]
		for _, w := range sess.windows {
			for _, id := range slices.Sorted(maps.Keys(w.panes)) {
				p := w.panes[id]
				kind := st.agentKind(p)
				if kind == "" {
					continue
				}
				state := p.agentNow()
				if state != p.agentWas || p.agentSince.IsZero() {
					p.agentWas, p.agentSince = state, now
				}
				out = append(out, agentInfo(sess, w, p, kind))
			}
		}
	}
	return out
}

// agentInfo describes the agent of kind in pane p of window w.
func agentInfo(sess *session, w *window, p *pane, kind string) api.AgentInfo {
	a := api.AgentInfo{
		Session: sess.name, Window: w.index, WindowName: w.name, Pane: p.id, Kind: kind,
		Profile: p.profile, Folder: p.folder(), State: p.agentWas, Since: p.agentSince,
	}
	if p.chat != nil {
		snap := &p.chat.snap
		a.Model, a.Cost = snap.Model, snap.Cost
		if a.State == api.AgentNeedsYou {
			a.Waits = api.AgentWaitsApproval
			if snap.Question != nil {
				a.Waits = api.AgentWaitsQuestion
			}
		}
	}
	return a
}

// syncAgents sends the agent list to every client when it differs from
// the one they last heard. Call it after any change to sessions, windows,
// panes, or agent states.
func (st *state) syncAgents() {
	list := st.agents()
	if slices.EqualFunc(list, st.agentsSent, sameAgent) {
		return
	}
	before := st.agentsSent
	st.agentsSent = list
	m := &api.AgentList{Agents: list}
	for c := range st.clients {
		st.enqueue(c, m)
	}
	st.notifyChanges(before, list)
}

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
// pane name, and no text from the agent.
func agentWord(kind string) string {
	if kind == api.AgentKindChat {
		return "Snow chat"
	}
	return "Claude Code"
}

// sameAgent reports whether a and b say the same.
func sameAgent(a, b api.AgentInfo) bool {
	sa, sb := a.Since, b.Since
	a.Since, b.Since = time.Time{}, time.Time{}
	return a == b && sa.Equal(sb)
}

// focusOnAttach focuses pane id of sess and shows its window, for a client
// that attaches to answer that agent. A pane sess does not have, or 0,
// changes nothing.
func (st *state) focusOnAttach(sess *session, id int) {
	if id == 0 {
		return
	}
	w, p := sess.findPane(id)
	if p == nil {
		return
	}
	if err := st.applyToPane(sess, w, p, &api.Command{Op: api.OpFocus, Target: id}); err != nil {
		st.srv.log.Info("focus on attach", "pane", id, "err", err)
		return
	}
	st.repaint(sess)
	st.changed(sess)
}
