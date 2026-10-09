package server

import (
	"errors"
	"net"
	"time"

	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/pkg/api"
)

// outboxSize bounds queued messages per client. Screen updates stop at half
// of it (see flushPane); a client whose control messages overflow it is
// disconnected instead of stalling the model goroutine.
const outboxSize = 256

// client is one connected snow client. The model goroutine owns every field
// except conn, which only the client's own goroutines use.
type client struct {
	conn       *api.Conn
	raw        net.Conn
	out        chan api.Message
	cols, rows int
	session    string           // attached session; empty until Attach
	last       map[int][]string // pane ID → rows last sent
	seen       map[int]uint64   // pane ID → pane Version last sent
	dropped    bool
}

// serveConn handles one connection until it closes.
func (s *Server) serveConn(raw net.Conn) {
	conn := api.NewConn(raw)
	hello, err := api.ServerHandshake(conn, api.Welcome{ServerVersion: s.opts.Version, Protocol: api.ProtocolVersion})
	if err != nil {
		s.log.Info("handshake failed", "err", err)
		_ = raw.Close()
		return
	}
	c := &client{conn: conn, raw: raw, out: make(chan api.Message, s.outbox)}
	// A bad size is not fatal: the client gets the default size until it
	// sends a valid Resize.
	if err := hello.Validate(); err == nil {
		c.cols, c.rows = hello.Cols, hello.Rows
	} else {
		s.log.Info("ignored hello size", "err", err)
	}
	if !s.call(func(st *state) { st.clients[c] = struct{}{} }) {
		_ = raw.Close()
		return
	}
	go c.write()
	s.read(c)
	s.call(func(st *state) { st.dropClient(c) })
}

// write sends queued messages until the outbox closes, then closes the
// connection.
func (c *client) write() {
	defer func() { _ = c.raw.Close() }()
	for m := range c.out {
		if err := c.conn.Send(m); err != nil {
			return
		}
	}
}

// read handles requests until the connection ends.
func (s *Server) read(c *client) {
	for {
		m, err := c.conn.Receive()
		if errors.Is(err, api.ErrUnknownType) || errors.Is(err, api.ErrBadPayload) {
			continue
		}
		if err != nil {
			return
		}
		switch m := m.(type) {
		case *api.Attach:
			s.call(func(st *state) { st.attach(c, m) })
		case *api.Resize:
			s.call(func(st *state) { st.resize(c, m) })
		case *api.Input:
			s.input(c, m)
		}
	}
}

// enqueue queues m for c, or drops c if its outbox is full.
func (st *state) enqueue(c *client, m api.Message) {
	if c.dropped {
		return
	}
	select {
	case c.out <- m:
	default:
		st.srv.log.Warn("client too slow; disconnecting", "session", c.session)
		st.dropClient(c)
	}
}

// lingerTimeout bounds how long a closing client may take to receive its
// last messages.
const lingerTimeout = 5 * time.Second

// dropClient forgets c and closes its connection at once. Closing the
// connection, not only the outbox, matters: a writer blocked on a peer that
// stopped reading would otherwise keep the connection, and its read loop,
// open.
func (st *state) dropClient(c *client) {
	st.forget(c)
	_ = c.raw.Close()
}

// closeClient forgets c and closes its connection after the messages
// already queued are written, or after lingerTimeout.
func (st *state) closeClient(c *client) {
	st.forget(c)
	_ = c.raw.SetWriteDeadline(time.Now().Add(lingerTimeout))
}

// forget removes c from the server and closes its outbox; its writer then
// closes the connection.
func (st *state) forget(c *client) {
	if c.dropped {
		return
	}
	c.dropped = true
	delete(st.clients, c)
	close(c.out)
	if sess, ok := st.sessions[c.session]; ok {
		st.fitSession(sess) // the session may grow back
	}
}

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
