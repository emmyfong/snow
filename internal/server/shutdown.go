package server

import (
	"net"
	"time"

	"github.com/emmyfong/snow/pkg/api"
)

// shutdownWait bounds how long a client on another protocol has to ask the
// server to stop before its connection closes.
const shutdownWait = 2 * time.Second

// awaitShutdown gives a client on another protocol one more message: a
// Shutdown stops the server; anything else, or nothing, only closes the
// connection. This is how a newer client restarts an older server.
func (s *Server) awaitShutdown(conn *api.Conn, raw net.Conn) {
	defer func() { _ = raw.Close() }()
	if err := raw.SetReadDeadline(time.Now().Add(shutdownWait)); err != nil {
		return
	}
	if m, err := conn.Receive(); err == nil {
		if _, ok := m.(*api.Shutdown); ok {
			s.shutdown()
		}
	}
}

// shutdown stops the server at a client's request: Done closes, Run
// returns, and closing the server saves every session first.
func (s *Server) shutdown() {
	s.log.Info("asked to stop; saving sessions")
	// Done closes on the model goroutine, like every other close of it,
	// so two closes cannot race.
	s.call(func(*state) {
		select {
		case <-s.done:
		default:
			close(s.done)
		}
	})
}
