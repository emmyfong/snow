package server

import (
	"testing"

	"github.com/emmyfong/snow/internal/term"
)

// pane returns the term.Pane of the focused pane in a session's shown
// window, for tests.
func (s *Server) pane(t *testing.T, session string) *term.Pane {
	t.Helper()
	var p *term.Pane
	s.call(func(st *state) {
		if sess, ok := st.sessions[session]; ok {
			w := sess.shown()
			p = w.panes[w.focused].term
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
