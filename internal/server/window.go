package server

import (
	"errors"
	"slices"

	"github.com/emmyfong/snow/internal/layout"
)

// statusRows is the height of the status bar the client draws below the
// panes. Panes are laid out in the rest of the session.
const statusRows = 1

// window is one tab of a session: a layout tree and the panes in it.
type window struct {
	index   int
	name    string
	tree    *layout.Tree
	panes   map[int]*pane
	focused int // pane ID
}

// area is the size panes are laid out in: the session minus the status bar.
func (sess *session) area() (cols, rows int) {
	return sess.cols, max(sess.rows-statusRows, 1)
}

// window returns the window with index, or nil.
func (sess *session) window(index int) *window {
	for _, w := range sess.windows {
		if w.index == index {
			return w
		}
	}
	return nil
}

// shown returns the active window.
func (sess *session) shown() *window { return sess.window(sess.active) }

// freeIndex returns the lowest window index not in use, starting at 0 as in
// tmux.
func (sess *session) freeIndex() int {
	for i := 0; ; i++ {
		if sess.window(i) == nil {
			return i
		}
	}
}

// findPane returns the window and pane with ID id, or nils.
func (sess *session) findPane(id int) (*window, *pane) {
	for _, w := range sess.windows {
		if p, ok := w.panes[id]; ok {
			return w, p
		}
	}
	return nil, nil
}

// newWindow starts a window with one pane running profile in dir, adds it
// to sess at the lowest free index, and shows it.
func (st *state) newWindow(sess *session, profile, dir string) (*window, error) {
	cols, rows := sess.area()
	id := st.nextPane
	p, err := st.srv.startPane(id, cols, rows, profile, dir, "")
	if err != nil {
		return nil, err
	}
	st.nextPane++
	w := &window{
		index:   sess.freeIndex(),
		name:    p.profile,
		tree:    layout.New(layout.PaneID(id)),
		panes:   map[int]*pane{id: p},
		focused: id,
	}
	sess.windows = append(sess.windows, w)
	slices.SortFunc(sess.windows, func(a, b *window) int { return a.index - b.index })
	sess.active = w.index
	return w, nil
}

// splitPane splits pane target of w on side d and starts profile in dir in
// the new pane, which gets the focus. A split that would make a pane too
// small, or a program that fails to start, leaves the window unchanged.
func (st *state) splitPane(sess *session, w *window, target int, d layout.Direction, profile, dir string) error {
	cols, rows := sess.area()
	id := st.nextPane
	zoomed := w.tree.Zoomed // Split unzooms; a failed start puts it back
	if err := w.tree.SplitWithin(layout.PaneID(target), d, layout.PaneID(id), cols, rows); err != nil {
		return err
	}
	r := layout.Rects(w.tree, cols, rows).Panes[layout.PaneID(id)]
	p, err := st.srv.startPane(id, r.W, r.H, profile, dir, "")
	if err != nil {
		// The split succeeded on a tree with more than one pane, so closing
		// the new pane cannot fail.
		_ = w.tree.Close(layout.PaneID(id))
		w.tree.Zoomed = zoomed
		return err
	}
	st.nextPane++
	w.panes[id] = p
	w.focused = id
	return nil
}

// removePane closes pane id of w and takes it out of the tree. The last pane
// closes the window instead.
func (st *state) removePane(sess *session, w *window, id int) {
	p := w.panes[id]
	order := w.tree.Panes()
	err := w.tree.Close(layout.PaneID(id))
	if errors.Is(err, layout.ErrLastPane) {
		st.removeWindow(sess, w)
		return
	}
	delete(w.panes, id)
	st.closePane(p)
	if w.focused == id {
		// Focus the pane that came before the closed one, as tmux does when
		// the last-active pane is gone.
		i := max(slices.Index(order, layout.PaneID(id))-1, 0)
		w.focused = int(w.tree.Panes()[min(i, len(w.tree.Panes())-1)])
	}
}

// removeWindow closes w and its panes. The next window is shown, or the
// previous one when w was last. The last window ends the session.
func (st *state) removeWindow(sess *session, w *window) {
	i := slices.Index(sess.windows, w)
	sess.windows = slices.Delete(sess.windows, i, i+1)
	for _, p := range w.panes {
		st.closePane(p)
	}
	if len(sess.windows) == 0 {
		st.endSession(sess)
		return
	}
	if sess.active == w.index {
		sess.active = sess.windows[min(i, len(sess.windows)-1)].index
	}
}

// closePane stops p's program without blocking the model goroutine (ADR
// 0007): Close waits for the program and the pane's goroutines. The pane is
// already gone from the state, and its exit notice then finds nothing.
// Server.Close waits for these closes.
func (st *state) closePane(p *pane) {
	st.srv.closing.Add(1)
	go func() {
		defer st.srv.closing.Done()
		_ = p.term.Close()
	}()
}

// relayout resizes every visible pane of sess to its rectangle. Panes hidden
// by zoom keep their size until they show again.
func (st *state) relayout(sess *session) {
	cols, rows := sess.area()
	for _, w := range sess.windows {
		for id, r := range layout.Rects(w.tree, cols, rows).Panes {
			p := w.panes[int(id)]
			if c, r2 := p.term.Size(); c == max(r.W, 1) && r2 == max(r.H, 1) {
				continue
			}
			if err := p.term.Resize(max(r.W, 1), max(r.H, 1)); err != nil {
				st.srv.log.Warn("resize pane", "session", sess.name, "pane", id, "err", err)
			}
		}
	}
}
