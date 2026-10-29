package server

import (
	"errors"
	"net"

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
	// chatLive holds the chat panes c is in step with: it got their
	// snapshot and every event since. Others get a snapshot next.
	chatLive map[int]bool
	// wantsModels says c asked for the model list and hears when a fetch
	// finishes.
	wantsModels bool
	// focused is set while c's terminal reports focus. The server skips a
	// desktop notification for the pane such a client shows.
	focused bool
	dropped bool
}

// serveConn handles one connection until it closes.
func (s *Server) serveConn(raw net.Conn) {
	conn := api.NewConn(raw)
	hello, err := api.ServerHandshake(conn, api.Welcome{ServerVersion: s.opts.Version, Protocol: api.ProtocolVersion})
	if errors.Is(err, api.ErrVersionMismatch) {
		s.log.Info("client on another protocol", "err", err)
		s.awaitShutdown(conn, raw)
		return
	}
	if err != nil {
		s.log.Info("handshake failed", "err", err)
		_ = raw.Close()
		return
	}
	c := &client{conn: conn, raw: raw, out: make(chan api.Message, s.outbox), chatLive: map[int]bool{}}
	// A bad size is not fatal: the client gets the default size until it
	// sends a valid Resize.
	if err := hello.Validate(); err == nil {
		c.cols, c.rows = hello.Cols, hello.Rows
	} else {
		s.log.Info("ignored hello size", "err", err)
	}
	if !s.call(func(st *state) {
		st.clients[c] = struct{}{}
		st.served = true
		// A client that has not attached yet, such as the home page, needs
		// the sessions to choose from.
		st.enqueue(c, st.listMessage())
		st.enqueue(c, &api.AgentList{Agents: st.agents()})
	}) {
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
			snap, err := s.loadSaved(m.Session)
			if err != nil {
				s.reply(c, &api.Error{Code: api.CodeAttachFailed, Message: err.Error()})
				continue
			}
			s.call(func(st *state) { st.attach(c, m, snap) })
		case *api.Resize:
			s.call(func(st *state) { st.resize(c, m) })
		case *api.Input:
			s.input(c, m)
		case *api.Mouse:
			s.mouse(c, m)
		case *api.Command:
			s.call(func(st *state) { st.command(c, m) })
		case *api.ScrollbackRequest:
			s.scrollback(c, m)
		case *api.SearchRequest:
			s.search(c, m)
		case *api.ChatInput:
			s.chatInput(c, m)
		case *api.ModelListRequest:
			s.modelList(c, m)
		case *api.ConversationListRequest:
			s.conversations(c, m)
		case *api.AuthListRequest:
			s.authList(c)
		case *api.LoginRequest:
			s.login(c, m)
		case *api.LogoutRequest:
			s.logout(c, m)
		case *api.Detach:
			s.call(func(st *state) { st.detach(c) })
		case *api.KillSession:
			s.call(func(st *state) { st.killRequest(c, m) })
		case *api.RenameSession:
			s.call(func(st *state) { st.renameRequest(c, m) })
		case *api.AgentState:
			s.call(func(st *state) { st.agentState(m) })
		case *api.ClientFocus:
			s.call(func(st *state) { st.clientFocus(c, m) })
		case *api.Shutdown:
			s.shutdown()
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

// dropClient forgets c and closes its connection at once. Closing the
// connection, not only the outbox, matters: a writer blocked on a peer that
// stopped reading would otherwise keep the connection, and its read loop,
// open.
func (st *state) dropClient(c *client) {
	st.forget(c)
	_ = c.raw.Close()
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
		st.sessionsChanged()
	}
	st.maybeDone()
}
