package server

import (
	"fmt"
	"strconv"

	"github.com/emmyfong/snow/internal/layout"
	"github.com/emmyfong/snow/pkg/api"
)

// openFile shows cmd's path in the viewer of its window. It selects the
// file in the files pane when the window has one, opens the viewer beside
// target when it has none, and gives the viewer the conversation's change
// evidence for the file.
func (st *state) openFile(sess *session, cmd *api.Command) error {
	w, _ := sess.findPane(cmd.Target)
	if w == nil {
		return fmt.Errorf("no window holds pane %d", cmd.Target)
	}
	rel := cmd.Args[api.ArgPath]
	if !validTreePath(rel) {
		return fmt.Errorf("open-file: %q is not a path below the folder", rel)
	}
	line := 0
	if s := cmd.Args[api.ArgLine]; s != "" {
		line, _ = strconv.Atoi(s)
	}
	profile, folder := st.newPaneArgs(sess, cmd, cmd.Target)
	if fid, ok := w.paneOfKind(api.PaneFiles); ok {
		if fp := w.panes[fid]; fp != nil && fp.folder() == folder {
			if err := fp.side.Act(api.FilesActSelect, map[string]string{api.ArgPath: rel}); err != nil {
				st.srv.log.Warn("select the file in the files pane", "pane", fid, "err", err)
			}
		}
	}
	evidence := windowEvidence(w, logicalAbs(folder, rel))
	vid, ok := w.paneOfKind(api.PaneViewer)
	if ok {
		vp := w.panes[vid]
		if vp.folder() != folder {
			return fmt.Errorf("the viewer shows %s, not %s", vp.folder(), folder)
		}
		if w.tree.Zoomed != 0 && int(w.tree.Zoomed) != vid {
			w.tree.Unzoom()
		}
		w.focused = vid
		sess.active = w.index
		v, ok := vp.side.(*viewerPane)
		if !ok {
			return fmt.Errorf("viewer pane %d is not a viewer", vid)
		}
		return v.open(rel, line, evidence)
	}
	if err := st.splitPane(sess, w, cmd.Target, layout.Right, api.PaneViewer, profile, folder); err != nil {
		return fmt.Errorf("open the viewer: %w", err)
	}
	vp := w.panes[w.focused]
	v, ok := vp.side.(*viewerPane)
	if !ok {
		return fmt.Errorf("the new viewer pane is not a viewer")
	}
	return v.open(rel, line, evidence)
}
