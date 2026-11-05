package server

import (
	"fmt"
	"slices"
	"strings"

	"github.com/emmyfong/snow/internal/layout"
	"github.com/emmyfong/snow/pkg/api"
)

// openKind runs OpOpen on pane p of window w: it focuses the pane of kind
// that w already has, or splits p and opens one on its right, focused. The
// new pane takes p's profile and folder unless cmd names others.
func (st *state) openKind(sess *session, w *window, p *pane, cmd *api.Command) error {
	kind := cmd.Args[api.ArgKind]
	if !st.srv.kinds.has(kind) {
		return fmt.Errorf("%w %q. You can open: %s", ErrUnknownKind, kind, strings.Join(st.srv.kinds.names(), ", "))
	}
	if id, ok := w.paneOfKind(kind); ok {
		if w.tree.Zoomed != 0 && int(w.tree.Zoomed) != id {
			w.tree.Unzoom()
		}
		w.focused = id
		sess.active = w.index
		return nil
	}
	profile, folder := st.newPaneArgs(sess, cmd, p.id)
	return st.splitPane(sess, w, p.id, layout.Right, kind, profile, folder)
}

// paneOfKind returns the lowest-numbered pane of kind in w, so the choice
// does not depend on map order.
func (w *window) paneOfKind(kind string) (int, bool) {
	var ids []int
	for id, p := range w.panes {
		if p.kind() == kind {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return 0, false
	}
	return slices.Min(ids), true
}

// has reports whether kind is registered.
func (k *paneKinds) has(kind string) bool {
	return slices.ContainsFunc(k.list, func(info api.PaneKindInfo) bool { return info.Name == kind })
}

// names returns the registered kinds' names, in order.
func (k *paneKinds) names() []string {
	out := make([]string, len(k.list))
	for i, info := range k.list {
		out[i] = info.Name
	}
	return out
}
