package server

import (
	"github.com/emmyfong/snow/internal/store"
	"time"

	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/pkg/api"
)

// attach shows a session to c: a running one, a saved one restored from
// snap, or a new one when asked.
func (st *state) attach(c *client, m *api.Attach, snap *store.Snapshot) {
	if err := m.Validate(); err != nil {
		st.enqueue(c, &api.Error{Code: api.CodeBadCommand, Message: err.Error()})
		return
	}
	sess, ok := st.sessions[m.Session]
	if !ok && snap != nil {
		cols, rows := c.cols, c.rows
		if cols <= 0 || rows <= 0 {
			cols, rows = defaultCols, defaultRows
		}
		var err error
		if sess, err = st.restore(snap, cols, rows); err != nil {
			st.srv.log.Warn("restore failed", "session", m.Session, "err", err)
			st.enqueue(c, &api.Error{Code: api.CodeAttachFailed, Message: err.Error()})
			return
		}
		st.sessions[sess.name] = sess
		st.served = true
		st.save(sess)
		st.srv.log.Info("session restored", "session", sess.name)
		ok = true
	}
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
	st.enqueue(c, st.profileList())
	st.fitSession(sess)
	st.sessionsChanged()
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
		if st.endedKey(c, in.Pane, in.Key) {
			return
		}
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

var mouseButtons = map[string]term.Button{
	api.ButtonNone: term.ButtonNone, api.ButtonLeft: term.ButtonLeft, api.ButtonMiddle: term.ButtonMiddle,
	api.ButtonRight: term.ButtonRight, api.ButtonWheelUp: term.ButtonWheelUp, api.ButtonWheelDown: term.ButtonWheelDown,
}

var mouseActions = map[string]term.Action{
	api.ActionPress: term.ActionPress, api.ActionRelease: term.ActionRelease, api.ActionMotion: term.ActionMotion,
}

// mouse sends a mouse event to a pane of c's session. Like input, it only
// finds the pane on the model goroutine. The pane itself drops the event
// when its program did not ask for the mouse.
func (s *Server) mouse(c *client, m *api.Mouse) {
	if err := m.Validate(); err != nil {
		s.log.Info("ignored mouse event", "err", err)
		return
	}
	var tp *term.Pane
	s.call(func(st *state) {
		if p := st.paneOf(c, m.Pane); p != nil {
			tp = p.term
		}
	})
	if tp == nil {
		return
	}
	tp.SendMouse(term.Mouse{X: m.X, Y: m.Y, Button: mouseButtons[m.Button], Action: mouseActions[m.Action]})
}

// paneOf returns the pane with id in c's attached session, or nil. A client
// never reaches panes of other sessions.
func (st *state) paneOf(c *client, id int) *pane {
	sess, ok := st.sessions[c.session]
	if !ok || c.dropped {
		return nil
	}
	_, p := sess.findPane(id)
	return p
}

// profileList names the profiles new panes can run.
func (st *state) profileList() *api.ProfileList {
	names := st.srv.launch.profiles()
	l := &api.ProfileList{Profiles: names}
	if len(names) > 0 {
		l.Default = names[0]
	}
	return l
}
