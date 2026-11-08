package server

import (
	"errors"
	"fmt"

	"github.com/emmyfong/snow/pkg/api"
)

// openFile shows cmd's path in the files pane of its window. Until the
// viewer exists, that means selecting the file and opening its folders.
func (st *state) openFile(sess *session, cmd *api.Command) error {
	w, _ := sess.findPane(cmd.Target)
	if w == nil {
		return fmt.Errorf("no window holds pane %d", cmd.Target)
	}
	id, ok := w.paneOfKind(api.PaneFiles)
	if !ok {
		return errors.New("this window has no files pane")
	}
	p := w.panes[id]
	if p == nil || p.side == nil {
		return fmt.Errorf("files pane %d is gone", id)
	}
	return p.side.Act(api.FilesActSelect, map[string]string{api.ArgPath: cmd.Args[api.ArgPath]})
}
