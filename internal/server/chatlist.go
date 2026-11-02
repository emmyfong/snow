package server

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/pkg/api"
)

// Bounds on a conversation list, so it always fits a frame: a title, folder,
// or error is cut to maxListText characters, and items stop at listBudget
// bytes, newest first. A variable so a test can shrink it.
const (
	maxListText  = 200
	maxListModel = 80
)

var listBudget = snapshotBudget

// cutRunes returns s cut to at most n characters, with an ellipsis.
func cutRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

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
	// A chat with no message yet is not a conversation to pick up: every new
	// session and /new makes one. A file that cannot be read stays, so the
	// user can see it.
	entries = slices.DeleteFunc(entries, func(e convstore.Entry) bool { return e.Err == nil && e.Turns == 0 })
	list := &api.ConversationList{Pane: req.Pane, All: req.All, Items: make([]api.ConversationInfo, 0, len(entries))}
	size := 0
	for _, e := range entries {
		it := api.ConversationInfo{
			ID: e.ID, Title: cutRunes(e.Title, maxListText), Folder: cutRunes(e.Folder, maxListText),
			Model: cutRunes(e.Model, maxListModel), LastActivity: e.LastActivity, Turns: e.Turns, Open: open[e.ID],
		}
		if e.Err != nil {
			it.Err = cutRunes(e.Err.Error(), maxListText)
		}
		b, err := json.Marshal(it)
		if err != nil || size+len(b)+1 > listBudget {
			break
		}
		size += len(b) + 1
		list.Items = append(list.Items, it)
	}
	list.Truncated = len(entries) - len(list.Items)
	s.reply(c, list)
}
