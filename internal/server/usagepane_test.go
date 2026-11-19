package server

import (
	"testing"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/store"
	"github.com/emmyfong/snow/pkg/api"
)

// openSidePane splits a pane of kind off pane id and returns the new
// pane's ID once ready accepts its state.
func openSidePane(t *testing.T, c *apitest.Client, id int, kind string, ready func(*api.SidePaneState) bool) int {
	t.Helper()
	c.Send(&api.Command{Op: api.OpOpen, Target: id, Args: map[string]string{api.ArgKind: kind}})
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	for _, p := range shownPanes(l) {
		if p.Kind == kind {
			c.WaitSide(p.ID, ready)
			return p.ID
		}
	}
	t.Fatalf("no %s pane", kind)
	return 0
}

// openUsagePane splits a usage pane off chat pane id and returns the new
// pane's ID once its usage has arrived.
func openUsagePane(t *testing.T, c *apitest.Client, id int) int {
	t.Helper()
	return openSidePane(t, c, id, api.PaneUsage, func(s *api.SidePaneState) bool {
		var st api.UsageState
		return api.DecodeSideState(s, &st) == nil && st.Totals.Input > 0
	})
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

// waitRestoredSide waits for the restored side pane of kind in session name
// to publish a state that ready accepts, and returns it.
func waitRestoredSide(t *testing.T, s *Server, name, kind string, ready func(api.SideState) bool) api.SideState {
	t.Helper()
	var out api.SideState
	eventually(t, "the restored "+kind+" pane", func() bool {
		s.call(func(st *state) {
			sess, ok := st.sessions[name]
			if !ok {
				return
			}
			for _, w := range sess.windows {
				for _, p := range w.panes {
					if p.sideKind != kind || p.side == nil {
						continue
					}
					if st := p.side.State(); st != nil && ready(st) {
						out = st
						return
					}
				}
			}
		})
		return out != nil
	})
	return out
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
	var name string
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
		name = sess.name
		st.sessions[sess.name] = sess
	})
	if saved.Kind != api.PaneUsage || saved.Conversation != conv {
		t.Fatalf("saved pane %+v; want the usage kind on %q", saved, conv)
	}
	if name == "" {
		t.Fatal("restore did not start a session")
	}
	// The restored pane rebuilds its totals from the conversation file, not
	// from the field it opened with.
	st := waitRestoredSide(t, s, name, api.PaneUsage, func(st api.SideState) bool {
		u, ok := st.(*api.UsageState)
		return ok && u.Totals.Input > 0
	}).(*api.UsageState)
	if st.Conversation != conv {
		t.Fatalf("restored pane follows %q, want %q", st.Conversation, conv)
	}
	if st.Totals.Input != 10 || st.Totals.Output != 2 {
		t.Fatalf("restored totals %+v; want the saved turn's 10 in and 2 out", st.Totals)
	}
}
