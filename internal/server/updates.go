package server

import (
	"github.com/emmyfong/snow/internal/layout"
	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/pkg/api"
)

// fitSession sizes a session to the smallest of its attached clients, as
// tmux does, so every client can show the whole session. When the size
// changes, the panes are laid out again and every attached client gets the
// new layout.
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
	st.changed(sess)
}

// changed lays out the panes of sess again and sends the layout to every
// client attached to it. Call it after any change to windows or panes.
func (st *state) changed(sess *session) {
	st.relayout(sess)
	st.save(sess)
	for c := range st.clients {
		if c.session == sess.name {
			st.enqueue(c, st.layout(sess))
		}
	}
}

// repaint makes the next frame send every visible row to the clients of
// sess, for example after the shown window changes.
func (st *state) repaint(sess *session) {
	for c := range st.clients {
		if c.session == sess.name {
			c.last = map[int][]string{}
			c.seen = map[int]uint64{}
		}
	}
}

// layout describes every window of sess and where its visible panes are.
// A zoomed window lists only its zoomed pane.
func (st *state) layout(sess *session) *api.Layout {
	cols, rows := sess.area()
	l := &api.Layout{Session: sess.name, Active: sess.active}
	for _, w := range sess.windows {
		info := api.WindowInfo{Index: w.index, Name: w.name, Zoomed: w.tree.Zoomed != 0}
		g := layout.Rects(w.tree, cols, rows)
		for _, id := range w.tree.Panes() {
			r, ok := g.Panes[id]
			if !ok {
				continue
			}
			p := w.panes[int(id)]
			info.Panes = append(info.Panes, api.PaneInfo{
				ID:      int(id),
				Rect:    api.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H},
				Profile: p.profile,
				Folder:  p.term.Folder(),
				Focused: int(id) == w.focused,
			})
		}
		l.Windows = append(l.Windows, info)
	}
	return l
}

// flush sends each attached client the rows that changed since its last
// update, for the panes it can see. It runs on every frame tick. A pane
// whose output is being written at that moment is busy: flush draws every
// free pane first, then waits for the busy ones, each for at most one feed
// piece. So a pane that only echoed a key never waits behind panes that
// print without pause, and those still update on every frame.
func (st *state) flush() {
	var busy []func()
	for c := range st.clients {
		sess, ok := st.sessions[c.session]
		if !ok {
			continue
		}
		w := sess.shown()
		if w == nil {
			continue
		}
		cols, rows := sess.area()
		for id := range layout.Rects(w.tree, cols, rows).Panes {
			if p := w.panes[int(id)]; !st.flushPane(c, p, false) {
				busy = append(busy, func() { st.flushPane(c, p, true) })
			}
		}
	}
	for _, f := range busy {
		f()
	}
}

// flushPane sends c the rows of p that changed since the last update c was
// sent. A client with a half-full outbox is skipped; c.last stays as it was,
// so the next update it gets carries every row changed in between. Without
// wait it reads the pane only if it is free, and reports false for a busy
// pane, to be drawn again with wait.
func (st *state) flushPane(c *client, p *pane, wait bool) bool {
	if len(c.out) >= cap(c.out)/2 {
		return true
	}
	v := p.term.Version()
	if prev, ok := c.seen[p.id]; ok && prev == v {
		return true
	}
	var f term.Frame
	if wait {
		f = p.term.Frame()
	} else {
		var ok bool
		if f, ok = p.term.TryFrame(); !ok {
			return false
		}
	}
	rows := f.Lines
	last := c.last[p.id]
	var changed []api.LineUpdate
	for i, row := range rows {
		if i >= len(last) || last[i] != row {
			changed = append(changed, api.LineUpdate{Row: i, Text: row})
		}
	}
	c.last[p.id] = rows
	c.seen[p.id] = v
	st.enqueue(c, &api.PaneUpdate{Pane: p.id, Lines: changed, Cursor: &api.Cursor{X: f.X, Y: f.Y, Visible: f.Visible}, Mouse: f.Mouse})
	return true
}
