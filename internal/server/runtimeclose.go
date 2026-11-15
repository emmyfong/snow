package server

import (
	"errors"
	"fmt"

	"github.com/emmyfong/snow/pkg/api"
)

// deferRuntimeClose leaves the view in place while its runtime stops. A
// failure must remain visible even when the local process has already exited.
func (st *state) deferRuntimeClose(c *client, sess *session, cmd *api.Command) bool {
	var panes []*pane
	var originalWindow *window
	switch cmd.Op {
	case api.OpClose:
		_, p := sess.findPane(cmd.Target)
		if p != nil {
			panes = append(panes, p)
		}
	case api.OpCloseWindow:
		if w := sess.window(cmd.Target); w != nil {
			originalWindow = w
			for _, p := range w.panes {
				panes = append(panes, p)
			}
		}
	default:
		return false
	}
	type closingChat struct {
		pane *pane
		chat *chatPane
	}
	var chats []closingChat
	var conversations []conversation
	for _, p := range panes {
		if p.chat != nil && p.chat.snap.Runtime != nil {
			if p.chat.closing {
				return true
			}
			chats = append(chats, closingChat{pane: p, chat: p.chat})
			conversations = append(conversations, p.chat.conv)
		}
	}
	if len(chats) == 0 {
		return false
	}
	for _, p := range chats {
		p.chat.closing = true
	}
	request := *cmd
	request.Args = nil // close commands carry no mutable options on this path
	st.srv.closing.Add(1)
	go func() {
		defer st.srv.closing.Done()
		var stopErr error
		for _, conv := range conversations {
			stopErr = errors.Join(stopErr, conv.Close())
		}
		st.srv.send(func(current *state) {
			if current.sessions[sess.name] != sess {
				return
			}
			for _, p := range panes {
				_, live := sess.findPane(p.id)
				if live != p {
					return
				}
			}
			if stopErr != nil {
				for _, p := range chats {
					// Close keeps its final snapshot available after a cleanup failure.
					// The controller already emitted the detailed runtime failure.
					current.chatEvent(&api.ChatEvent{Pane: p.pane.id, Agent: p.chat.snap.Agent, Kind: api.ChatEventError,
						Error: &api.ChatError{Code: api.ChatErrRuntimeStop, Message: fmt.Sprintf("Chat remains open: runtime stop was not confirmed: %v", stopErr)}})
				}
				return
			}
			w := sess.window(request.Target)
			unchangedWindow := request.Op == api.OpCloseWindow && w == originalWindow && len(w.panes) == len(panes)
			if unchangedWindow {
				for _, p := range panes {
					if w.panes[p.id] != p {
						unchangedWindow = false
						break
					}
				}
			}
			if request.Op == api.OpCloseWindow && !unchangedWindow {
				// A later split was not part of the confirmed close. Remove only
				// the original panes, whose runtimes have now stopped.
				for _, p := range panes {
					window, _ := sess.findPane(p.id)
					current.removePane(sess, window, p.id)
				}
				current.enqueue(c, &api.Error{Code: api.CodeNotice, Message: "Closed the original panes. New panes opened during shutdown were kept."})
			} else if err := current.apply(sess, &request); err != nil {
				current.enqueue(c, &api.Error{Code: api.CodeCommandFailed, Message: err.Error()})
				return
			}
			if current.sessions[sess.name] == sess {
				current.repaint(sess)
				current.changed(sess)
				current.rememberLayout(sess)
			}
		})
	}()
	return true
}
