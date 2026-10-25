package server

import (
	"errors"
	"fmt"

	"github.com/emmyfong/snow/internal/agent"
	"github.com/emmyfong/snow/pkg/api"
)

// chatInput hands a ChatInput to its pane's conversation. The model
// goroutine only finds the conversation; the call happens on c's reader
// goroutine, so it keeps the order of c's inputs and never blocks the
// model goroutine.
func (s *Server) chatInput(c *client, in *api.ChatInput) {
	if err := in.Validate(); err != nil {
		s.refuse(c, api.CodeBadCommand, err)
		return
	}
	var conv *agent.Conversation
	queued := 0
	s.call(func(st *state) {
		if p := st.paneOf(c, in.Pane); p != nil && p.chat != nil && string(p.chat.conv.Agent()) == in.Agent {
			conv, queued = p.chat.conv, len(p.chat.snap.Queue)
		}
	})
	if conv == nil {
		s.refuse(c, api.CodeCommandFailed, fmt.Errorf("no agent %s in pane %d", in.Agent, in.Pane))
		return
	}
	var err error
	switch in.Kind {
	case api.InputMessage:
		// The queue is read from the snapshot, which can trail the agent
		// by a message or two; the bound still holds the queue near
		// MaxChatQueue, well inside a frame.
		if queued >= api.MaxChatQueue {
			s.refuse(c, api.CodeCommandFailed, fmt.Errorf("%d messages already wait for the turn to end", queued))
			return
		}
		err = conv.Send(in.Text)
	case api.InputAnswer:
		err = conv.Answer(agent.Answer{ID: in.ID, Decision: agent.Decision(in.Decision), Reason: in.Text})
	case api.InputInterrupt:
		err = conv.Interrupt()
	case api.InputRetry:
		err = conv.Retry()
	}
	// A pane closed meanwhile has nothing to report to.
	if err != nil && !errors.Is(err, agent.ErrClosed) {
		s.refuse(c, api.CodeCommandFailed, err)
	}
}
