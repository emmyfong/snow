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
	st.sendLayout(sess)
	// A closed pane may have been an agent that waited for the user, or
	// one that sent no hooks.
	st.syncNeedsYou(sess)
	st.syncHint()
	st.syncAgents()
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
		info := api.WindowInfo{Index: w.index, Name: w.name, Zoomed: w.tree.Zoomed != 0, AgentState: windowAgent(w)}
		g := layout.Rects(w.tree, cols, rows)
		for _, id := range w.tree.Panes() {
			r, ok := g.Panes[id]
			if !ok {
				continue
			}
			p := w.panes[int(id)]
			folder := p.dir // a chat's folder
			if p.term != nil {
				folder = p.term.Folder()
			}
			info.Panes = append(info.Panes, api.PaneInfo{
				ID:         int(id),
				Kind:       p.kind(),
				Rect:       api.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H},
				Profile:    p.profile,
				Folder:     folder,
				Focused:    int(id) == w.focused,
				AgentState: p.agentNow(),
			})
		}
		l.Windows = append(l.Windows, info)
	}
	return l
}

// flush sends each attached client the rows that changed since its last
// update, for the panes it can see. It runs on every frame tick and never
// waits: a pane whose output is being written at that moment is busy, and
// a short goroutine reads it instead, handing the frame back when it has
// it (fetchFrame). So the model goroutine never blocks on a pane (ADR 0007),
// a pane that only echoed a key goes out at once, and panes that print
// without pause still update about once a frame.
func (st *state) flush() {
	for c := range st.clients {
		sess, ok := st.sessions[c.session]
		if !ok {
			continue
		}
		st.syncChats(c)
		w := sess.shown()
		if w == nil {
			continue
		}
		cols, rows := sess.area()
		for id := range layout.Rects(w.tree, cols, rows).Panes {
			st.flushPane(c, w.panes[int(id)])
		}
	}
}

// flushPane sends c the rows of p that changed since the last update c was
// sent, if p is free; a busy pane is read off the model goroutine. A client
// with a half-full outbox is skipped; c.last stays as it was, so the next
// update it gets carries every row changed in between.
func (st *state) flushPane(c *client, p *pane) {
	if c.dropped || len(c.out) >= cap(c.out)/2 || p.term == nil {
		return
	}
	v := p.term.Version()
	if prev, ok := c.seen[p.id]; ok && prev == v {
		return
	}
	f, ok := p.term.TryFrame()
	if !ok {
		st.fetchFrame(p)
		return
	}
	st.sendFrame(c, p, v, f)
}

// fetchFrame reads busy pane p on its own goroutine, which waits for the
// pane's lock, and hands the frame to the model goroutine for every client
// that shows p. One read at a time per pane: a pane already being read is
// left alone.
func (st *state) fetchFrame(p *pane) {
	if p.fetching {
		return
	}
	p.fetching = true
	srv := st.srv
	go func() {
		v := p.term.Version()
		f := p.term.Frame()
		srv.post(func(st *state) {
			p.fetching = false
			st.frameReady(p, v, f)
		})
	}()
}

// frameReady sends frame f of p, read at version v or later, to every
// client still showing p that has not seen v.
func (st *state) frameReady(p *pane, v uint64, f term.Frame) {
	for c := range st.clients {
		sess, ok := st.sessions[c.session]
		if !ok || c.dropped || len(c.out) >= cap(c.out)/2 {
			continue
		}
		if w := sess.shown(); w == nil || w.panes[p.id] != p {
			continue
		}
		if prev, ok := c.seen[p.id]; ok && prev >= v {
			continue
		}
		st.sendFrame(c, p, v, f)
	}
}

// sendFrame sends c the rows of frame f of p that differ from what c has.
func (st *state) sendFrame(c *client, p *pane, v uint64, f term.Frame) {
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
}
