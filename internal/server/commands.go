package server

import (
	"errors"
	"fmt"

	"github.com/emmyfong/snow/internal/layout"
	"github.com/emmyfong/snow/pkg/api"
)

// directions maps the protocol's directions to the layout's.
var directions = map[string]layout.Direction{
	api.DirLeft: layout.Left, api.DirRight: layout.Right, api.DirUp: layout.Up, api.DirDown: layout.Down,
}

// opposite maps each direction to the one facing it.
var opposite = map[layout.Direction]layout.Direction{
	layout.Left: layout.Right, layout.Right: layout.Left, layout.Up: layout.Down, layout.Down: layout.Up,
}

// command applies cmd to c's session and sends the new layout to every
// client of the session. A command that fails changes nothing and is
// answered with an error.
func (st *state) command(c *client, cmd *api.Command) {
	sess, ok := st.sessions[c.session]
	if !ok || c.dropped {
		return
	}
	if err := cmd.Validate(); err != nil {
		st.enqueue(c, &api.Error{Code: api.CodeBadCommand, Message: err.Error()})
		return
	}
	if err := st.apply(sess, cmd); err != nil {
		st.enqueue(c, &api.Error{Code: api.CodeCommandFailed, Message: err.Error()})
		return
	}
	if _, ok := st.sessions[sess.name]; !ok {
		return // the command closed the last window
	}
	// Clients drop the screens of panes a layout hides (zoom, another
	// window), so every change resends the visible screens. Commands come
	// at human speed, so the cost is small.
	st.repaint(sess)
	st.changed(sess)
}

// apply performs one validated command on sess.
func (st *state) apply(sess *session, cmd *api.Command) error {
	switch cmd.Op {
	case api.OpNewWindow:
		profile, dir := st.newPaneArgs(sess, cmd, 0)
		_, err := st.newWindow(sess, profile, dir)
		return err
	case api.OpRenameWindow, api.OpSelectWindow, api.OpCloseWindow:
		w := sess.window(cmd.Target)
		if w == nil {
			return fmt.Errorf("no window %d", cmd.Target)
		}
		switch cmd.Op {
		case api.OpRenameWindow:
			w.name = cmd.Args[api.ArgName]
		case api.OpSelectWindow:
			sess.active = w.index
		default:
			st.removeWindow(sess, w)
		}
		return nil
	case api.OpNextWindow, api.OpPrevWindow:
		step := 1
		if cmd.Op == api.OpPrevWindow {
			step = -1
		}
		for i, w := range sess.windows {
			if w.index == sess.active {
				sess.active = sess.windows[(i+step+len(sess.windows))%len(sess.windows)].index
				return nil
			}
		}
		return nil
	}
	w, p := sess.findPane(cmd.Target)
	if p == nil {
		return fmt.Errorf("no pane %d", cmd.Target)
	}
	return st.applyToPane(sess, w, p, cmd)
}

// applyToPane performs a pane command on pane p of window w.
func (st *state) applyToPane(sess *session, w *window, p *pane, cmd *api.Command) error {
	id := layout.PaneID(p.id)
	cols, rows := sess.area()
	dir := directions[cmd.Args[api.ArgDir]]
	switch cmd.Op {
	case api.OpSplit:
		profile, folder := st.newPaneArgs(sess, cmd, p.id)
		return st.splitPane(sess, w, p.id, dir, profile, folder)
	case api.OpClose:
		st.removePane(sess, w, p.id)
		return nil
	case api.OpResize:
		err := w.tree.ResizeCells(id, dir, cmd.Cells(), cols, rows)
		if errors.Is(err, layout.ErrNoEdge) {
			// At the window border, move the other edge the same way, as
			// tmux does: Ctrl+Left shrinks a pane on the left.
			err = w.tree.ResizeCells(id, opposite[dir], -cmd.Cells(), cols, rows)
		}
		if errors.Is(err, layout.ErrNoEdge) {
			return nil // a lone pane has no edge to move
		}
		return err
	case api.OpZoom:
		if w.tree.Zoomed == id {
			w.tree.Unzoom()
			return nil
		}
		w.focused = p.id
		return w.tree.Zoom(id)
	case api.OpSwap:
		other, err := layout.Neighbor(w.tree, id, dir, cols, rows)
		if err != nil {
			return err
		}
		return w.tree.Swap(id, other)
	case api.OpFocus:
		target := id
		if _, ok := cmd.Args[api.ArgDir]; ok {
			// Leaving a zoomed pane unzooms, as tmux does, so the focused
			// pane is always on screen.
			w.tree.Unzoom()
			n, err := layout.Neighbor(w.tree, id, dir, cols, rows)
			if errors.Is(err, layout.ErrNoNeighbor) {
				return nil
			}
			if err != nil {
				return err
			}
			target = n
		}
		if w.tree.Zoomed != 0 && w.tree.Zoomed != target {
			w.tree.Unzoom()
		}
		w.focused = int(target)
		sess.active = w.index
		return nil
	case api.OpEven:
		return w.tree.Even(id)
	}
	return fmt.Errorf("%w: %s is not a pane command", api.ErrInvalidCommand, cmd.Op)
}

// newPaneArgs returns the profile and folder for a new pane: the command's
// arguments, or else those of pane from (the shown window's focused pane
// when from is 0).
func (st *state) newPaneArgs(sess *session, cmd *api.Command, from int) (profile, folder string) {
	profile, folder = cmd.Args[api.ArgProfile], cmd.Args[api.ArgFolder]
	if from == 0 {
		if w := sess.shown(); w != nil {
			from = w.focused
		}
	}
	if _, p := sess.findPane(from); p != nil {
		if profile == "" {
			profile = p.profile
		}
		if folder == "" {
			folder = p.term.Folder()
		}
	}
	return profile, folder
}
