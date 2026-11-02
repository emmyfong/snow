package server

import (
	"fmt"

	"github.com/emmyfong/snow/internal/term"
	"github.com/emmyfong/snow/pkg/api"
)

// send_keys serves an agent in a pane. The owner goroutine looks the pane up
// and marks that the agent types into it; the bytes go to the PTY here, on
// this connection's goroutine, so a program that stops reading never stalls
// the owner (ADR 0007).

// sendKeys types into a pane for an agent in another pane.
func (s *Server) sendKeys(c *client, r *api.SendKeysRequest) {
	if err := r.Validate(); err != nil {
		s.refuse(c, api.CodeBadCommand, err)
		return
	}
	var (
		tp *term.Pane
		e  *api.Error
	)
	if !s.call(func(st *state) {
		sess, p, err := st.toolPane(r.Caller, r.Pane)
		if err != nil {
			e = err
			return
		}
		switch {
		case p.id == r.Caller:
			e = &api.Error{Code: api.CodeOwnPane, Message: fmt.Sprintf("pane %d is your own pane; send_keys does not type into the pane the agent runs in", p.id)}
		case p.ended:
			e = &api.Error{Code: api.CodePaneEnded, Message: fmt.Sprintf("pane %d ended and waits for the user to press r; send_keys does not restart panes", p.id)}
		case st.isAgentPane(p):
			e = &api.Error{Code: api.CodeAgentPane, Message: fmt.Sprintf("pane %d runs an agent; send_keys does not type into another agent's pane", p.id)}
		default:
			tp = p.term
			st.markDriven(sess, p)
		}
	}) {
		return // the server stopped, so no reply can reach the client
	}
	if e != nil {
		s.reply(c, e)
		return
	}
	before := tp.DroppedInput()
	if r.Text != "" {
		tp.Paste(r.Text)
	}
	for _, k := range r.Keys {
		tp.SendKey(toTermKey(k))
	}
	s.reply(c, &api.SendKeys{Pane: r.Pane, Dropped: int(tp.DroppedInput() - before)})
}
