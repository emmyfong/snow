package server

import (
	"fmt"

	"github.com/emmyfong/snow/pkg/api"
)

// The pane tools serve an agent in a pane, through snow mcp: each request
// names the agent's pane as its caller, and reaches only the terminal
// panes of the caller's session. The model goroutine only looks panes up;
// reads, writes, and waits run outside it (ADR 0007).

// callerSession returns the session that holds pane caller, or an Error
// with CodeNoPane.
func (st *state) callerSession(caller int) (*session, *api.Error) {
	for _, sess := range st.sessions {
		if _, p := sess.findPane(caller); p != nil {
			return sess, nil
		}
	}
	return nil, &api.Error{Code: api.CodeNoPane, Message: fmt.Sprintf("your pane %d is not in this Snow server", caller)}
}

// isAgentPane reports whether an agent runs in p: a pane started from an
// agent profile, or one whose program sent a hook.
func (st *state) isAgentPane(p *pane) bool {
	return p.sawHook || st.srv.launch.isAgent(p.profile)
}

// panes answers a PanesRequest with the terminal panes of the caller's
// session.
func (s *Server) panes(c *client, r *api.PanesRequest) {
	if err := r.Validate(); err != nil {
		s.refuse(c, api.CodeBadCommand, err)
		return
	}
	s.call(func(st *state) {
		sess, e := st.callerSession(r.Caller)
		if e != nil {
			st.enqueue(c, e)
			return
		}
		st.enqueue(c, st.paneList(sess, r.Caller))
	})
}

// paneList lists the terminal panes of sess, panes a zoom hides
// included, in window order and, in a window, in layout order.
func (st *state) paneList(sess *session, caller int) *api.Panes {
	out := &api.Panes{Session: sess.name, Caller: caller}
	cols, rows := sess.area()
	for _, w := range sess.windows {
		rects := unzoomedRects(w.tree, cols, rows)
		for _, id := range w.tree.Panes() {
			p := w.panes[int(id)]
			if p == nil || p.term == nil {
				continue
			}
			r := rects[id]
			out.Panes = append(out.Panes, api.PaneSummary{
				ID:         p.id,
				Window:     w.index,
				WindowName: w.name,
				Shown:      w.index == sess.active,
				Profile:    p.profile,
				Folder:     p.folder(),
				Rect:       api.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H},
				Focused:    p.id == w.focused,
				Agent:      st.isAgentPane(p),
				AgentState: p.agentNow(),
				Ended:      p.ended,
			})
		}
	}
	return out
}
