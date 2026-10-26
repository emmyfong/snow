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
	snapModel := ""
	oldModel, oldEffort := "", ""
	s.call(func(st *state) {
		oldModel, oldEffort = st.defaultModel, st.defaultEffort
		if p := st.paneOf(c, in.Pane); p != nil && p.chat != nil && string(p.chat.conv.Agent()) == in.Agent {
			conv, queued, snapModel = p.chat.conv, len(p.chat.snap.Queue), p.chat.snap.Model
		}
	})
	if conv == nil {
		s.refuse(c, api.CodeCommandFailed, fmt.Errorf("no agent %s in pane %d", in.Agent, in.Pane))
		return
	}
	var err error
	switch in.Kind {
	case api.ChatInputNew:
		err = s.newChat(c, in.Pane)
	case api.ChatInputResume:
		err = s.resumeChat(c, in.Pane, in.Conversation)
	case api.ChatInputMessage:
		if snapModel == "" {
			s.refuse(c, api.CodeCommandFailed, errors.New("no model: choose one before sending"))
			return
		}
		// The queue is read from the snapshot, which can trail the agent
		// by a message or two; the bound still holds the queue near
		// MaxChatQueue, well inside a frame.
		if queued >= api.MaxChatQueue {
			s.refuse(c, api.CodeCommandFailed, fmt.Errorf("%d messages already wait for the turn to end", queued))
			return
		}
		err = conv.Send(in.Text)
	case api.ChatInputAnswer:
		err = conv.Answer(agent.Answer{ID: in.ID, Decision: agent.Decision(in.Decision), Reason: in.Text})
	case api.ChatInputInterrupt:
		err = conv.Interrupt()
	case api.ChatInputRetry:
		err = conv.Retry()
	case api.ChatInputModel:
		if err = s.switchModel(c, in.Pane, conv, in.Model, in.Effort); err == nil && in.SaveDefault {
			s.saveDefault(c, in, oldModel, oldEffort)
		}
	case api.ChatInputEffort:
		// The check can wait for a provider's model list. It runs apart from
		// the reader goroutine, so this client's Esc is not held up behind it.
		go func() { s.finishInput(c, in, conv, s.setEffort(c, in.Pane, conv, snapModel, in.Effort)) }()
		return
	case api.ChatInputRename:
		err = conv.Rename(in.Text)
	case api.ChatInputMode:
		err = conv.SetMode(agent.Mode(in.Mode))
	}
	s.finishInput(c, in, conv, err)
}

// finishInput tells c why an input failed. A conversation that closed
// because its pane closed has nothing to report to; one that closed because
// the pane now shows another conversation lost the user's input, and c is
// told so.
func (s *Server) finishInput(c *client, in *api.ChatInput, conv *agent.Conversation, err error) {
	switch {
	case err == nil:
	case !errors.Is(err, agent.ErrClosed):
		s.refuse(c, api.CodeCommandFailed, err)
	default:
		replaced := false
		s.call(func(st *state) {
			p := st.paneOf(c, in.Pane)
			replaced = p != nil && p.chat != nil && p.chat.conv != conv
		})
		if replaced {
			s.refuse(c, api.CodeCommandFailed, errors.New("this pane now shows another conversation. Send again"))
		}
	}
}

// saveDefault makes the chosen model the default for new chats and writes
// it to the config file. The write runs off the model goroutine; when it
// fails, the default it replaced comes back and c is told why.
func (s *Server) saveDefault(c *client, in *api.ChatInput, oldModel, oldEffort string) {
	o := s.opts.Chat
	if o == nil || o.SaveDefault == nil {
		s.refuse(c, api.CodeCommandFailed, errors.New("saving a default is not set up"))
		return
	}
	s.call(func(st *state) {
		st.defaultModel, st.defaultEffort = in.Model, in.Effort
	})
	go func() {
		if err := o.SaveDefault(in.Model, in.Effort); err != nil {
			s.send(func(st *state) {
				if st.defaultModel == in.Model && st.defaultEffort == in.Effort {
					st.defaultModel, st.defaultEffort = oldModel, oldEffort
				}
			})
			s.refuse(c, api.CodeCommandFailed, fmt.Errorf("save the default model: %w", err))
		}
	}()
}
