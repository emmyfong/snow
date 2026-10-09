package server

import (
	"errors"
	"net"
	"time"

	"github.com/emmyfong/snow/pkg/api"
)

// outboxSize bounds queued messages per client. A client that falls this far
// behind is disconnected instead of stalling the model goroutine.
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
	c := &client{conn: conn, raw: raw, out: make(chan api.Message, outboxSize), cols: hello.Cols, rows: hello.Rows}
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
			s.call(func(st *state) { st.resize(c, m.Cols, m.Rows) })
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

// dropClient forgets c and closes its outbox, which closes the connection.
func (st *state) dropClient(c *client) {
	if c.dropped {
		return
	}
	c.dropped = true
	delete(st.clients, c)
	close(c.out)
}

// attach shows a session to c, creating it if asked.
func (st *state) attach(c *client, m *api.Attach) {
	sess, ok := st.sessions[m.Session]
	if !ok && !m.Create {
		st.enqueue(c, &api.Error{Code: api.CodeNoSession, Message: "no session named " + m.Session})
		return
	}
	if !ok {
		var err error
		if sess, err = st.createSession(m.Session, c.cols, c.rows); err != nil {
			st.enqueue(c, &api.Error{Code: api.CodeAttachFailed, Message: err.Error()})
			return
		}
	}
	c.session = sess.name
	c.last = map[int][]string{}
	c.seen = map[int]uint64{}
	sess.lastUsed = time.Now()
	st.fitSession(sess)
	st.enqueue(c, st.layout(sess))
}

func (st *state) resize(c *client, cols, rows int) {
	c.cols, c.rows = cols, rows
	if sess, ok := st.sessions[c.session]; ok {
		st.fitSession(sess)
		st.enqueue(c, st.layout(sess))
	}
}
