package server

import (
	"testing"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/store"
	"github.com/emmyfong/snow/pkg/api"
)

// openContextPane splits a context pane off chat pane id and returns the
// new pane's ID once its first full state arrives.
func openContextPane(t *testing.T, c *apitest.Client, id int) int {
	t.Helper()
	return openSidePane(t, c, id, api.PaneContext, func(s *api.SidePaneState) bool {
		var st api.ContextState
		return api.DecodeSideState(s, &st) == nil && st.Occupancy.Known && st.Occupancy.Tokens > 0
	})
}

// contextState decodes a context pane's state.
func contextState(t *testing.T, s *api.SidePaneState) api.ContextState {
	t.Helper()
	var st api.ContextState
	if err := api.DecodeSideState(s, &st); err != nil {
		t.Fatal(err)
	}
	return st
}

// TestContextPaneFollowsChat: a context pane opened from a chat follows
// that chat's conversation, and its occupancy comes from the saved records.
func TestContextPaneFollowsChat(t *testing.T) {
	s, _, _ := newChatServer(t, reply("hi"))
	c := connect(t, s, 100, 30)
	chat := openChat(t, c)
	say(c, chat, "one")
	c.WaitChat(chat, idle)
	conv := c.Chat(chat).Conversation
	if conv == "" {
		t.Fatal("chat has no conversation")
	}

	cp := openContextPane(t, c, chat)
	st := contextState(t, c.Side(cp))
	if st.Conversation != conv {
		t.Fatalf("context follows %q, want the chat's %q", st.Conversation, conv)
	}
	if !st.Occupancy.Known || st.Occupancy.Tokens != 10 {
		t.Fatalf("occupancy %+v; want the last prompt's 10 tokens", st.Occupancy)
	}

	// A second client joining the session rebuilds the same pane state from
	// the server.
	c2 := connect(t, s, 100, 30)
	c2.Send(&api.Attach{Session: "work"})
	nextLayout(t, c2, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	st2 := contextState(t, c2.WaitSide(cp, func(s *api.SidePaneState) bool {
		var st api.ContextState
		return api.DecodeSideState(s, &st) == nil && !st.Loading
	}))
	if st2.Conversation != conv || !st2.Occupancy.Known {
		t.Fatalf("second client state %+v; want the same conversation and occupancy", st2)
	}
}

// TestContextPaneSavedAndRestored: a saved session keeps the conversation
// a context pane follows, and a restore opens it again on that
// conversation.
func TestContextPaneSavedAndRestored(t *testing.T) {
	s, _, _ := newChatServer(t, reply("hi"))
	c := connect(t, s, 100, 30)
	chat := openChat(t, c)
	say(c, chat, "one")
	c.WaitChat(chat, idle)
	conv := c.Chat(chat).Conversation
	cp := openContextPane(t, c, chat)

	var saved store.Pane
	var name string
	s.call(func(st *state) {
		snap := st.snapshot(st.sessions["work"])
		// Restore under another name, so the running session keeps its
		// conversation file open until the server closes it.
		snap.Name = "restored"
		for _, w := range snap.Windows {
			for _, sp := range w.Panes {
				if sp.ID == cp {
					saved = sp
				}
			}
		}
		if saved.Kind != api.PaneContext || saved.Conversation != conv {
			return
		}
		sess, err := st.restore(&snap, nil, nil, 100, 30)
		if err != nil {
			t.Errorf("restore: %v", err)
			return
		}
		name = sess.name
		st.sessions[sess.name] = sess
	})
	if saved.Kind != api.PaneContext || saved.Conversation != conv {
		t.Fatalf("saved pane %+v; want the context kind on %q", saved, conv)
	}
	if name == "" {
		t.Fatal("restore did not start a session")
	}
	// The restored pane rebuilds its occupancy from the conversation file,
	// not from the field it opened with.
	st := waitRestoredSide(t, s, name, api.PaneContext, func(st api.SideState) bool {
		c, ok := st.(*api.ContextState)
		return ok && c.Occupancy.Known && c.Occupancy.Tokens > 0
	}).(*api.ContextState)
	if st.Conversation != conv {
		t.Fatalf("restored pane follows %q, want %q", st.Conversation, conv)
	}
	if st.Occupancy.Tokens != 10 {
		t.Fatalf("restored occupancy %+v; want the saved prompt's 10 tokens", st.Occupancy)
	}
}
