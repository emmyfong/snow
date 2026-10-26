package server

import (
	"fmt"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/pkg/api"
)

// conversations answers a ConversationListRequest with the past
// conversations of the pane's folder, or of every folder. It runs on c's
// reader goroutine: reading the conversation folder takes disk time the
// model goroutine must not spend.
func (s *Server) conversations(c *client, req *api.ConversationListRequest) {
	if err := req.Validate(); err != nil {
		s.refuse(c, api.CodeBadCommand, err)
		return
	}
	o := s.opts.Chat
	if o == nil {
		s.refuse(c, api.CodeCommandFailed, ErrNoChat)
		return
	}
	var folder, profile string
	var found bool
	open := map[string]bool{}
	s.call(func(st *state) {
		if p := st.paneOf(c, req.Pane); p != nil && p.chat != nil {
			folder, profile, found = p.chat.folder, p.chat.profile, true
		}
		for _, sess := range st.sessions {
			for _, w := range sess.windows {
				for _, p := range w.panes {
					if p.chat != nil {
						open[p.chat.snap.Conversation] = true
					}
				}
			}
		}
	})
	if !found {
		s.refuse(c, api.CodeCommandFailed, fmt.Errorf("no chat in pane %d", req.Pane))
		return
	}
	var entries []convstore.Entry
	var err error
	if req.All {
		entries, err = o.Store.ListAll()
	} else {
		entries, err = o.Store.List(folder, profile)
	}
	if err != nil {
		s.refuse(c, api.CodeCommandFailed, err)
		return
	}
	list := &api.ConversationList{Pane: req.Pane, All: req.All, Items: make([]api.ConversationInfo, 0, len(entries))}
	for _, e := range entries {
		it := api.ConversationInfo{
			ID: e.ID, Title: e.Title, Folder: e.Folder, Model: e.Model,
			LastActivity: e.LastActivity, Turns: e.Turns, Open: open[e.ID],
		}
		if e.Err != nil {
			it.Err = e.Err.Error()
		}
		list.Items = append(list.Items, it)
	}
	s.reply(c, list)
}
