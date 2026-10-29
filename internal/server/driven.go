package server

import "time"

// drivenFor is how long a pane shows that an agent typed into it. A send
// finishes as soon as the program reads, so a mark tied to the call alone
// would flash too fast to see; it stays for drivenFor from the start.
const drivenFor = 2 * time.Second

// markDriven shows that an agent types into p, and clears the mark when
// drivenFor ends. It runs in the model goroutine.
func (st *state) markDriven(sess *session, p *pane) {
	deadline := time.Now().Add(drivenFor)
	p.drivenUntil = deadline
	st.sendLayout(sess)
	due := func() { st.srv.post(func(st *state) { st.drivenDone(sess, p, deadline) }) }
	if st.srv.opts.afterDriven != nil {
		st.srv.opts.afterDriven(due)
		return
	}
	time.AfterFunc(drivenFor, due)
}

// drivenDone clears the mark of p unless a later mark replaced it.
func (st *state) drivenDone(sess *session, p *pane, deadline time.Time) {
	if st.sessions[sess.name] != sess || p.drivenUntil.After(deadline) {
		return
	}
	p.drivenUntil = time.Time{}
	st.sendLayout(sess)
}

// driven reports whether an agent types into p now.
func (p *pane) driven() bool { return !p.drivenUntil.IsZero() }

// windowDriven reports whether an agent types into any pane of w, panes a
// zoom hides included.
func windowDriven(w *window) bool {
	for _, p := range w.panes {
		if p.driven() {
			return true
		}
	}
	return false
}
