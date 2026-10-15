package server

import (
	"fmt"

	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/pkg/api"
)

// findPane returns the terminal of pane id in c's session, or nil. The
// model goroutine only looks the pane up; callers read it outside, so a
// large read never stalls the other clients.
func (s *Server) findPane(c *client, id int) *term.Pane {
	var tp *term.Pane
	s.call(func(st *state) {
		if p := st.paneOf(c, id); p != nil {
			tp = p.term
		}
	})
	return tp
}

// reply queues m for c from c's reader goroutine. It waits for the model
// goroutine, so a client's replies keep the order of its requests.
func (s *Server) reply(c *client, m api.Message) {
	s.call(func(st *state) { st.enqueue(c, m) })
}

// refuse answers a history request that cannot be served.
func (s *Server) refuse(c *client, code string, err error) {
	s.reply(c, &api.Error{Code: code, Message: err.Error()})
}

// scrollback answers a ScrollbackRequest with a page of the pane's history.
func (s *Server) scrollback(c *client, r *api.ScrollbackRequest) {
	if err := r.Validate(); err != nil {
		s.refuse(c, api.CodeBadCommand, err)
		return
	}
	tp := s.findPane(c, r.Pane)
	if tp == nil {
		s.refuse(c, api.CodeCommandFailed, fmt.Errorf("no pane %d", r.Pane))
		return
	}
	lines, total := tp.History(r.From, r.Count)
	s.reply(c, &api.Scrollback{Pane: r.Pane, From: r.From, Total: total, Lines: lines})
}
