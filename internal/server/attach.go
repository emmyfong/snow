package server

import "github.com/emmyfong/snow/pkg/api"

// focusOnAttach focuses pane id of sess and shows its window, for a client
// that attaches to answer that agent. A pane sess does not have, or 0,
// changes nothing.
func (st *state) focusOnAttach(sess *session, id int) {
	if id == 0 {
		return
	}
	w, p := sess.findPane(id)
	if p == nil {
		return
	}
	if err := st.applyToPane(sess, w, p, &api.Command{Op: api.OpFocus, Target: id}); err != nil {
		st.srv.log.Info("focus on attach", "pane", id, "err", err)
		return
	}
	st.repaint(sess)
	st.changed(sess)
}
