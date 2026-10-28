package server

import (
	"maps"
	"slices"
	"time"

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
	st.agentsSent = list
	m := &api.AgentList{Agents: list}
	for c := range st.clients {
		st.enqueue(c, m)
	}
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
