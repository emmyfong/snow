package server

import (
	"testing"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/store"
	"github.com/emmyfong/snow/pkg/api"
)

// openUsagePane splits a usage pane off chat pane id and returns the new
// pane's ID once its first state arrives.
func openUsagePane(t *testing.T, c *apitest.Client, id int) int {
	t.Helper()
	c.Send(&api.Command{Op: api.OpOpen, Target: id, Args: map[string]string{api.ArgKind: api.PaneUsage}})
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	for _, p := range shownPanes(l) {
		if p.Kind == api.PaneUsage {
			c.WaitSide(p.ID, func(s *api.SidePaneState) bool {
				var st api.UsageState
				return api.DecodeSideState(s, &st) == nil && !st.Loading
			})
			return p.ID
		}
	}
	t.Fatal("no usage pane")
	return 0
}

// usageState decodes a usage pane's state.
func usageState(t *testing.T, s *api.SidePaneState) api.UsageState {
	t.Helper()
	var st api.UsageState
	if err := api.DecodeSideState(s, &st); err != nil {
		t.Fatal(err)
	}
	return st
}

// TestUsagePaneFollowsChat: a usage pane opened from a chat follows that
// chat's conversation, and its totals come from the saved records.
func TestUsagePaneFollowsChat(t *testing.T) {
	s, _, _ := newChatServer(t, reply("hi"), reply("again"))
	c := connect(t, s, 100, 30)
	chat := openChat(t, c)
	say(c, chat, "one")
	c.WaitChat(chat, idle)
	conv := c.Chat(chat).Conversation
	if conv == "" {
		t.Fatal("chat has no conversation")
	}

	up := openUsagePane(t, c, chat)
	st := usageState(t, c.WaitSide(up, func(s *api.SidePaneState) bool {
		var st api.UsageState
		return api.DecodeSideState(s, &st) == nil && st.Totals.Input > 0
	}))
	if st.Conversation != conv {
		t.Fatalf("usage follows %q, want the chat's %q", st.Conversation, conv)
	}
	if st.Turns != 1 || st.Totals.Input != 10 || st.Totals.Output != 2 {
		t.Fatalf("state %+v; want one turn of 10 in and 2 out", st)
	}

	// A second client joining the session rebuilds the same pane state from
	// the server, not from its own copy.
	c2 := connect(t, s, 100, 30)
	c2.Send(&api.Attach{Session: "work"})
	nextLayout(t, c2, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	st2 := usageState(t, c2.WaitSide(up, func(s *api.SidePaneState) bool {
		var st api.UsageState
		return api.DecodeSideState(s, &st) == nil && !st.Loading
	}))
	if st2.Conversation != conv || st2.Totals.Input != 10 {
		t.Fatalf("second client state %+v; want the same conversation and totals", st2)
	}
}

// TestUsagePaneSavedAndRestored: a saved session keeps the conversation a
// usage pane follows, and a restore opens it again on that conversation.
func TestUsagePaneSavedAndRestored(t *testing.T) {
	s, _, _ := newChatServer(t, reply("hi"))
	c := connect(t, s, 100, 30)
	chat := openChat(t, c)
	say(c, chat, "one")
	c.WaitChat(chat, idle)
	conv := c.Chat(chat).Conversation
	up := openUsagePane(t, c, chat)
	c.WaitSide(up, func(s *api.SidePaneState) bool {
		var st api.UsageState
		return api.DecodeSideState(s, &st) == nil && st.Totals.Input > 0
	})

	var saved store.Pane
	var restored *pane
	s.call(func(st *state) {
		snap := st.snapshot(st.sessions["work"])
		for _, w := range snap.Windows {
			for _, sp := range w.Panes {
				if sp.ID == up {
					saved = sp
				}
			}
		}
		if saved.Kind != api.PaneUsage || saved.Conversation != conv {
			return
		}
		sess, err := st.restore(&snap, nil, nil, 100, 30)
		if err != nil {
			t.Errorf("restore: %v", err)
			return
		}
		st.sessions[sess.name] = sess
		for _, w := range sess.windows {
			for _, rp := range w.panes {
				if rp.sideKind == api.PaneUsage {
					restored = rp
				}
			}
		}
	})
	if saved.Kind != api.PaneUsage || saved.Conversation != conv {
		t.Fatalf("saved pane %+v; want the usage kind on %q", saved, conv)
	}
	if restored == nil {
		t.Fatal("restore did not open a usage pane")
	}
	if got := restored.side.(conversationSide).Conversation(); got != conv {
		t.Fatalf("restored pane follows %q, want %q", got, conv)
	}
}
