package server

import (
	"testing"

	"github.com/emmyfong/snow/internal/term"
)

// pane returns the term.Pane of a session's first window, for tests.
func (s *Server) pane(t *testing.T, session string) *term.Pane {
	t.Helper()
	var p *term.Pane
	s.call(func(st *state) {
		if sess, ok := st.sessions[session]; ok {
			p = sess.windows[0].pane.term
		}
	})
	if p == nil {
		t.Fatalf("no session %q", session)
	}
	return p
}

// clientCount returns the number of connected clients, for tests.
func (s *Server) clientCount() int {
	n := 0
	s.call(func(st *state) { n = len(st.clients) })
	return n
}
