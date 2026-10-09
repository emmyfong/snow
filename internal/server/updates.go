package server

import (
	"github.com/emmyfong/snow/pkg/api"
)

// fitSession sizes a session to the smallest of its attached clients, as
// tmux does, so every client can show the whole pane. When the size changes,
// every attached client gets the new layout.
func (st *state) fitSession(sess *session) {
	cols, rows := 0, 0
	for c := range st.clients {
		if c.session != sess.name || c.cols <= 0 || c.rows <= 0 {
			continue
		}
		if cols == 0 || c.cols < cols {
			cols = c.cols
		}
		if rows == 0 || c.rows < rows {
			rows = c.rows
		}
	}
	if cols == 0 || (cols == sess.cols && rows == sess.rows) {
		return
	}
	sess.cols, sess.rows = cols, rows
	for c := range st.clients {
		if c.session == sess.name {
			st.enqueue(c, st.layout(sess))
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

// flushPane sends c the rows of p that changed since the last update c was
// sent. A client with a half-full outbox is skipped; c.last stays as it was,
// so the next update it gets carries every row changed in between.
func (st *state) flushPane(c *client, p *pane) {
	if len(c.out) >= cap(c.out)/2 {
		return
	}
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
	x, y, visible := p.term.Cursor()
	st.enqueue(c, &api.PaneUpdate{Pane: p.id, Lines: changed, Cursor: &api.Cursor{X: x, Y: y, Visible: visible}})
}
