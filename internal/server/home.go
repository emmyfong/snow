package server

import (
	"fmt"

	"github.com/emmyfong/snow/pkg/api"
)

// detach takes c out of its session without closing the connection, as the
// home page needs. The session keeps running.
func (st *state) detach(c *client) {
	prev := c.session
	c.session = ""
	c.last, c.seen = nil, nil
	if sess, ok := st.sessions[prev]; ok {
		st.fitSession(sess) // the session may grow back
	}
	// c is unattached now, so this sends it the list too.
	st.sessionsChanged()
}

// killRequest ends the session a client named.
func (st *state) killRequest(c *client, m *api.KillSession) {
	if err := m.Validate(); err != nil {
		st.enqueue(c, &api.Error{Code: api.CodeBadCommand, Message: err.Error()})
		return
	}
	if err := st.killSession(m.Session); err != nil {
		st.enqueue(c, &api.Error{Code: api.CodeCommandFailed, Message: err.Error()})
	}
}

// renameRequest renames a session and shows the new name to its clients.
func (st *state) renameRequest(c *client, m *api.RenameSession) {
	if err := m.Validate(); err != nil {
		st.enqueue(c, &api.Error{Code: api.CodeBadCommand, Message: err.Error()})
		return
	}
	sess, ok := st.sessions[m.Session]
	if !ok {
		st.enqueue(c, &api.Error{Code: api.CodeCommandFailed, Message: fmt.Sprintf("%v: %s", ErrNoSession, m.Session)})
		return
	}
	if m.Name == m.Session {
		return
	}
	if _, taken := st.sessions[m.Name]; taken {
		st.enqueue(c, &api.Error{Code: api.CodeCommandFailed, Message: "a session named " + m.Name + " already exists"})
		return
	}
	delete(st.sessions, sess.name)
	sess.name = m.Name
	st.sessions[sess.name] = sess
	for other := range st.clients {
		if other.session == m.Session {
			other.session = sess.name
			st.enqueue(other, st.layout(sess))
		}
	}
	st.sessionsChanged()
}

// maybeDone closes Done when nothing is left to serve: no session runs and
// no client is connected. A home page client with no session keeps the
// server up, so killing the last session there does not cut it off.
func (st *state) maybeDone() {
	if !st.served || len(st.sessions) > 0 || len(st.clients) > 0 {
		return
	}
	select {
	case <-st.srv.done:
	default:
		close(st.srv.done)
	}
}
