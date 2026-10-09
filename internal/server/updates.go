package server

import (
	"github.com/emmyfong/snow/pkg/api"
)

// fitSession sizes a session and its panes to its attached client.
func (st *state) fitSession(sess *session) {
	for c := range st.clients {
		if c.session == sess.name && c.cols > 0 && c.rows > 0 {
			sess.cols, sess.rows = c.cols, c.rows
		}
	}
	for _, w := range sess.windows {
		if cols, rows := w.pane.term.Size(); cols != sess.cols || rows != sess.rows {
			if err := w.pane.term.Resize(sess.cols, sess.rows); err != nil {
				st.srv.log.Warn("resize pane", "session", sess.name, "err", err)
			}
		}
	}
}

// layout describes sess: one window whose one pane fills the session.
func (st *state) layout(sess *session) *api.Layout {
	w := sess.windows[0]
	return &api.Layout{
		Session: sess.name,
		Active:  w.index,
		Windows: []api.WindowInfo{{
			Index: w.index,
			Name:  w.name,
			Panes: []api.PaneInfo{{
				ID:      w.pane.id,
				Rect:    api.Rect{W: sess.cols, H: sess.rows},
				Profile: w.pane.profile,
				Focused: true,
			}},
		}},
	}
}

// flush sends each attached client the rows that changed since its last
// update. It runs on every frame tick.
func (st *state) flush() {
	for c := range st.clients {
		sess, ok := st.sessions[c.session]
		if !ok {
			continue
		}
		for _, w := range sess.windows {
			st.flushPane(c, w.pane)
		}
	}
}

func (st *state) flushPane(c *client, p *pane) {
	v := p.term.Version()
	if prev, ok := c.seen[p.id]; ok && prev == v {
		return
	}
	rows := p.term.Lines()
	last := c.last[p.id]
	var changed []api.LineUpdate
	for i, row := range rows {
		if i >= len(last) || last[i] != row {
			changed = append(changed, api.LineUpdate{Row: i, Text: row})
		}
	}
	c.last[p.id] = rows
	c.seen[p.id] = v
	x, y := p.term.Cursor()
	st.enqueue(c, &api.PaneUpdate{Pane: p.id, Lines: changed, Cursor: &api.Cursor{X: x, Y: y, Visible: true}})
}
