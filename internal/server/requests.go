package server

import (
	"time"

	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/pkg/api"
)

// attach shows a session to c, creating it if asked.
func (st *state) attach(c *client, m *api.Attach) {
	sess, ok := st.sessions[m.Session]
	if !ok && !m.Create {
		msg := "no session named " + m.Session
		if m.Session == "" {
			msg = "no session name given"
		}
		st.enqueue(c, &api.Error{Code: api.CodeNoSession, Message: msg})
		return
	}
	if !ok {
		var err error
		if sess, err = st.createSession(m.Session, c.cols, c.rows, m.Dir); err != nil {
			st.srv.log.Warn("attach failed", "session", m.Session, "err", err)
			// The client runs as the same user, so the error's paths are
			// not secret, and they say what to fix.
			st.enqueue(c, &api.Error{Code: api.CodeAttachFailed, Message: err.Error()})
			return
		}
	}
	prev := c.session
	c.session = sess.name
	c.last = map[int][]string{}
	c.seen = map[int]uint64{}
	sess.lastUsed = time.Now()
	st.enqueue(c, st.layout(sess))
	st.fitSession(sess)
	if old, ok := st.sessions[prev]; ok && prev != sess.name {
		st.fitSession(old) // the session c left may grow back
	}
}

// resize records c's terminal size and refits its session. An invalid size
// is answered with an error and changes nothing.
func (st *state) resize(c *client, m *api.Resize) {
	if err := m.Validate(); err != nil {
		st.enqueue(c, &api.Error{Code: api.CodeBadSize, Message: err.Error()})
		return
	}
	c.cols, c.rows = m.Cols, m.Rows
	if sess, ok := st.sessions[c.session]; ok {
		st.fitSession(sess)
	}
}

// input sends a key or paste to a pane of c's session. The model goroutine
// only finds the pane; the write happens here, so a busy program cannot stall
// the model goroutine.
func (s *Server) input(c *client, in *api.Input) {
	if err := in.Validate(); err != nil {
		s.log.Info("ignored input", "err", err)
		return
	}
	var tp *term.Pane
	s.call(func(st *state) {
		if p := st.paneOf(c, in.Pane); p != nil {
			tp = p.term
		}
	})
	if tp == nil {
		return
	}
	if in.Key != nil {
		tp.SendKey(toTermKey(*in.Key))
		return
	}
	tp.Paste(in.Paste)
}

// paneOf returns the pane with id in c's attached session, or nil. A client
// never reaches panes of other sessions.
func (st *state) paneOf(c *client, id int) *pane {
	sess, ok := st.sessions[c.session]
	if !ok || c.dropped {
		return nil
	}
	for _, w := range sess.windows {
		if w.pane.id == id {
			return w.pane
		}
	}
	return nil
}
