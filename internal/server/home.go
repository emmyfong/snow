package server

import (
	"fmt"
	"github.com/emmyfong/snow/internal/store"

	"github.com/emmyfong/snow/pkg/api"
)

func (st *state) clientFocus(c *client, m *api.ClientFocus) {
	c.focused = m.Focused
}

// detach takes c out of its session without closing the connection, as the
// home page needs. The session keeps running.
func (st *state) detach(c *client) {
	prev := c.session
	unattach(c)
	if sess, ok := st.sessions[prev]; ok {
		st.fitSession(sess) // the session may grow back
	}
	// c is unattached now, so this sends it the list too.
	st.sessionsChanged()
}

// unattach leaves c connected but attached to no session, with no screen
// sent, so a later attach sends every screen in full.
func unattach(c *client) {
	c.session = ""
	c.focused = false
	c.last, c.seen = nil, nil
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
	info, saved := st.saved[m.Session]
	if !ok && !saved {
		st.enqueue(c, &api.Error{Code: api.CodeCommandFailed, Message: fmt.Sprintf("%v: %s", ErrNoSession, m.Session)})
		return
	}
	if m.Name == m.Session {
		return
	}
	_, taken := st.sessions[m.Name]
	if _, takenSaved := st.saved[m.Name]; taken || takenSaved {
		st.enqueue(c, &api.Error{Code: api.CodeCommandFailed, Message: "a session named " + m.Name + " already exists"})
		return
	}
	if !ok {
		st.renameSaved(c, info, m.Name)
		return
	}
	delete(st.sessions, sess.name)
	st.unsave(sess.name)
	sess.name = m.Name
	st.sessions[sess.name] = sess
	st.save(sess)
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

// renameSaved renames a saved session that is not running. The list shows
// the new name at once; the saver moves the file after the work queued
// before it, and a move that fails puts the old name back and tells c why.
func (st *state) renameSaved(c *client, info store.Info, name string) {
	old := info.Name
	delete(st.saved, old)
	info.Name = name
	st.saved[name] = info
	st.sessionsChanged()
	srv := st.srv
	srv.saver.Move(old, name, func(err error) {
		if err == nil {
			return
		}
		srv.post(func(st *state) {
			if _, ok := st.saved[name]; ok {
				delete(st.saved, name)
				info.Name = old
				st.saved[old] = info
			}
			st.sessionsChanged()
			st.enqueue(c, &api.Error{Code: api.CodeCommandFailed, Message: "rename " + old + ": " + err.Error()})
		})
	})
}
