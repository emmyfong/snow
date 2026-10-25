package server

import (
	"encoding/json"
	"fmt"
	"github.com/emmyfong/snow/internal/store"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providertest"
	"github.com/emmyfong/snow/internal/tool"
	"github.com/emmyfong/snow/pkg/api"
)

// chatModel is the model test chats start with.
const chatModel = "fake/model-1"

// chatOptions returns test options whose chats talk to a fake provider
// that plays replies.
func chatOptions(t *testing.T, replies ...providertest.Reply) (Options, *providertest.Fake, *convstore.Store) {
	t.Helper()
	store, err := convstore.Open(filepath.Join(t.TempDir(), "conversations"))
	if err != nil {
		t.Fatal(err)
	}
	fake := providertest.New("fake", replies...)
	reg := provider.NewRegistry()
	if err := reg.Register(fake); err != nil {
		t.Fatal(err)
	}
	opts := testOptions()
	opts.Chat = &ChatOptions{
		Store: store, Providers: reg, Tools: tool.NewRegistry(),
		Model: func() (string, string) { return chatModel, "" },
	}
	return opts, fake, store
}

func newChatServer(t *testing.T, replies ...providertest.Reply) (*Server, *providertest.Fake, *convstore.Store) {
	t.Helper()
	opts, fake, store := chatOptions(t, replies...)
	s := New(opts)
	t.Cleanup(s.Close)
	return s, fake, store
}

// reply is a whole reply of texts, one delta each.
func reply(texts ...string) providertest.Reply {
	evs := []provider.Event{&provider.Start{Model: "model-1"}}
	for _, s := range texts {
		evs = append(evs, &provider.TextDelta{Text: s})
	}
	evs = append(evs, &provider.Usage{Input: 10, Output: 2}, &provider.Stop{Reason: provider.StopEndTurn})
	return providertest.Reply{Events: evs}
}

// chatIn returns the ID of the first chat pane of l's shown window, or 0.
func chatIn(l *api.Layout) int {
	for _, p := range shownPanes(l) {
		if p.Kind == api.PaneChat {
			return p.ID
		}
	}
	return 0
}

// openChat attaches c to a new session "work" and splits its terminal
// pane with a chat on the right. It returns the chat pane's ID once c has
// the chat's snapshot.
func openChat(t *testing.T, c *apitest.Client) int {
	t.Helper()
	c.Send(&api.Attach{Session: "work", Create: true})
	l := nextLayout(t, c, func(*api.Layout) bool { return true })
	c.Send(&api.Command{Op: api.OpSplit, Target: shownPanes(l)[0].ID, Args: map[string]string{api.ArgDir: api.DirRight, api.ArgKind: api.PaneChat}})
	id := chatIn(nextLayout(t, c, func(l *api.Layout) bool { return chatIn(l) != 0 }))
	c.WaitChat(id, func(*api.ChatSnapshot) bool { return true })
	return id
}

// say sends text to chat pane id.
func say(c *apitest.Client, id int, text string) {
	agent := c.Chat(id).Agent
	c.Send(&api.ChatInput{Pane: id, Agent: agent, Kind: api.ChatInputMessage, Text: text})
}

// lastReply returns the text of the last assistant entry.
func lastReply(s *api.ChatSnapshot) string {
	for i := len(s.Entries) - 1; i >= 0; i-- {
		if s.Entries[i].Kind == api.ChatEntryAssistant {
			return s.Entries[i].Text
		}
	}
	return ""
}

func idle(s *api.ChatSnapshot) bool { return !s.Running && s.State == api.ChatStateIdle }

// TestChatTurn: a message gets a streamed reply, and the snapshot holds
// both, with the agent ID in every message.
func TestChatTurn(t *testing.T) {
	s, fake, _ := newChatServer(t, reply("hel", "lo"))
	c := connect(t, s, 100, 30)
	id := openChat(t, c)
	snap := c.Chat(id)
	if snap.Agent == "" || snap.Agent != snap.Conversation || snap.Model != chatModel || snap.Mode != "ask" {
		t.Fatalf("snapshot %+v", snap)
	}
	say(c, id, "hi")
	got := c.WaitChat(id, func(s *api.ChatSnapshot) bool { return lastReply(s) == "hello" && idle(s) })
	if got.Entries[0].Kind != api.ChatEntryUser || got.Entries[0].Text != "hi" || got.Usage == nil {
		t.Fatalf("chat %+v", got)
	}
	if reqs := fake.Requests(); len(reqs) != 1 || reqs[0].Model != "model-1" {
		t.Fatalf("requests %+v", reqs)
	}
}

// TestChatPaneLayout: split, zoom, swap, resize, focus, and close work
// with a chat pane next to a terminal pane, and in a window of its own.
func TestChatPaneLayout(t *testing.T) {
	s, _, _ := newChatServer(t)
	c := connect(t, s, 100, 30)
	id := openChat(t, c)
	term := 0
	for _, p := range shownPanes(currentLayout(s)) {
		if p.Kind == api.PaneTerminal {
			term = p.ID
		}
		if p.ID == id && (p.Folder == "" || p.Rect.W == 0) {
			t.Fatalf("chat pane %+v", p)
		}
	}
	steps := []api.Command{
		{Op: api.OpZoom, Target: id},
		{Op: api.OpZoom, Target: id},
		{Op: api.OpSwap, Target: id, Args: map[string]string{api.ArgDir: api.DirLeft}},
		{Op: api.OpResize, Target: id, Args: map[string]string{api.ArgDir: api.DirRight, api.ArgCells: "3"}},
		{Op: api.OpFocus, Target: term},
		{Op: api.OpSplit, Target: id, Args: map[string]string{api.ArgDir: api.DirDown}},
		{Op: api.OpEven, Target: id},
		{Op: api.OpClose, Target: id},
		{Op: api.OpNewWindow, Args: map[string]string{api.ArgKind: api.PaneChat}},
	}
	for _, cmd := range steps {
		c.Send(&cmd)
		nextLayout(t, c, func(*api.Layout) bool { return true })
	}
	l := currentLayout(s)
	if len(l.Windows) != 2 || len(shownPanes(l)) != 1 || shownPanes(l)[0].Kind != api.PaneChat {
		t.Fatalf("layout %+v", l)
	}
	if w := l.Windows[1]; w.Name != "chat" {
		t.Fatalf("chat window named %q", w.Name)
	}
	c.Send(&api.Command{Op: api.OpCloseWindow, Target: l.Active})
	l = nextLayout(t, c, func(*api.Layout) bool { return true })
	if len(l.Windows) != 1 {
		t.Fatalf("layout after close %+v", l)
	}
}

// currentLayout returns the layout of the session the server's client is
// attached to.
func currentLayout(s *Server) *api.Layout {
	var l *api.Layout
	s.call(func(st *state) {
		for cl := range st.clients {
			if sess, ok := st.sessions[cl.session]; ok {
				l = st.layout(sess)
			}
		}
	})
	return l
}

// TestChatDetachKeepsTurn: a turn runs on while no client is attached,
// and the snapshot on attach holds the whole reply.
func TestChatDetachKeepsTurn(t *testing.T) {
	hold := make(chan struct{})
	r := reply("hel", "lo")
	r.Hold = hold
	s, _, _ := newChatServer(t, r)
	c := connect(t, s, 100, 30)
	id := openChat(t, c)
	say(c, id, "hi")
	c.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Running })
	c.Send(&api.Detach{})
	c.Drain(200 * time.Millisecond)
	close(hold)
	eventually(t, "the turn to end while detached", func() bool {
		done := false
		s.call(func(st *state) {
			if _, p := st.findChat(id); p != nil {
				done = !p.chat.snap.Running && lastReply(&p.chat.snap) == "hello"
			}
		})
		return done
	})
	before := c.Snapshots
	c.Attach("work")
	got := c.WaitChat(id, func(*api.ChatSnapshot) bool { return c.Snapshots > before })
	if lastReply(got) != "hello" || got.Running {
		t.Fatalf("snapshot after attach %+v", got)
	}
}

// TestChatTwoClients: both clients of a session see the same transcript.
func TestChatTwoClients(t *testing.T) {
	s, _, _ := newChatServer(t, reply("one ", "two"))
	a := connect(t, s, 100, 30)
	id := openChat(t, a)
	b := connect(t, s, 100, 30)
	b.Attach("work")
	b.WaitChat(id, func(*api.ChatSnapshot) bool { return true })
	say(a, id, "count")
	for _, c := range []*apitest.Client{a, b} {
		c.WaitChat(id, func(s *api.ChatSnapshot) bool { return lastReply(s) == "one two" && idle(s) })
	}
	if a.ChatText(id) != b.ChatText(id) {
		t.Fatalf("transcripts differ:\n%s\n%s", a.ChatText(id), b.ChatText(id))
	}
}

// TestChatSlowClientResyncs: a client that stops reading does not hold up
// the other client and is not dropped; when it reads again, it rebuilds
// the whole reply from a fresh snapshot.
func TestChatSlowClientResyncs(t *testing.T) {
	var texts []string
	var want strings.Builder
	for i := range 60 {
		texts = append(texts, string(rune('a'+i%26)))
		want.WriteString(texts[i])
	}
	s, _, _ := newChatServer(t, reply(texts...))
	s.outbox = 8
	watch := connect(t, s, 100, 30)
	id := openChat(t, watch)
	slow := silentClient(t, s, "work")
	eventually(t, "the slow client to attach", func() bool {
		n := 0
		s.call(func(st *state) {
			for c := range st.clients {
				if c.session == "work" {
					n++
				}
			}
		})
		return n == 2
	})
	say(watch, id, "go")
	watch.WaitChat(id, func(s *api.ChatSnapshot) bool { return lastReply(s) == want.String() && idle(s) })
	if n := s.clientCount(); n != 2 {
		t.Fatalf("%d clients, want the slow one kept", n)
	}
	var snap *api.ChatSnapshot
	snaps := 0
	deadline := time.Now().Add(apitest.Wait)
	for time.Now().Before(deadline) && (snap == nil || lastReply(snap) != want.String() || !idle(snap)) {
		m, err := slow.Receive()
		if err != nil {
			t.Fatalf("slow client: %v", err)
		}
		switch m := m.(type) {
		case *api.ChatSnapshot:
			snaps++
			snap = m
		case *api.ChatEvent:
			if snap != nil {
				snap.Apply(m)
			}
		}
	}
	if snap == nil || lastReply(snap) != want.String() {
		t.Fatalf("slow client rebuilt %+v", snap)
	}
	if snaps < 2 {
		t.Fatalf("%d snapshots, want a fresh one after falling behind", snaps)
	}
}

// TestChatPaneIgnoresTerminalInput: keys and mouse events for a chat pane
// are dropped, and history requests are refused with a typed error.
func TestChatPaneIgnoresTerminalInput(t *testing.T) {
	s, _, _ := newChatServer(t)
	c := connect(t, s, 100, 30)
	id := openChat(t, c)
	c.Send(&api.Input{Pane: id, Paste: "echo hi\r"})
	c.Send(&api.Mouse{Pane: id, X: 1, Y: 1, Button: api.ButtonLeft, Action: api.ActionPress})
	for _, m := range []api.Message{&api.ScrollbackRequest{Pane: id, Count: 10}, &api.SearchRequest{Pane: id, Query: "x"}} {
		c.Send(m)
		deadline := time.Now().Add(apitest.Wait)
		var got *api.Error
		for got == nil && time.Now().Before(deadline) {
			r, _ := c.Next(time.Second)
			got, _ = r.(*api.Error)
		}
		if got == nil || got.Code != api.CodeNotTerminal {
			t.Fatalf("%T: got %+v, want %s", m, got, api.CodeNotTerminal)
		}
	}
	if snap := c.Chat(id); len(snap.Entries) != 0 {
		t.Fatalf("chat changed: %+v", snap)
	}
}

// TestChatCloseInterruptsTurn: closing a chat pane stops its turn and
// saves where it stopped; the conversation file stays.
func TestChatCloseInterruptsTurn(t *testing.T) {
	s, _, store := newChatServer(t, providertest.Reply{Events: []provider.Event{&provider.Start{Model: "m"}, &provider.TextDelta{Text: "part"}}, Wait: true})
	c := connect(t, s, 100, 30)
	id := openChat(t, c)
	conv := c.Chat(id).Conversation
	say(c, id, "hi")
	c.WaitChat(id, func(s *api.ChatSnapshot) bool { return lastReply(s) == "part" })
	c.Send(&api.Command{Op: api.OpClose, Target: id})
	nextLayout(t, c, func(l *api.Layout) bool { return chatIn(l) == 0 })
	eventually(t, "the interruption to be saved", func() bool {
		loaded, err := store.Load(conv)
		if err != nil {
			return false
		}
		for _, r := range loaded.Records {
			if _, ok := r.(*convstore.Interruption); ok {
				return true
			}
		}
		return false
	})
}

// TestChatSessionSavesWithoutError: a saved session leaves its chat panes
// out: a mixed window keeps its terminal panes, and a window of chats is
// not saved.
func TestChatSessionSavesWithoutError(t *testing.T) {
	opts, _, _ := chatOptions(t)
	opts.SessionsDir = t.TempDir()
	s := New(opts)
	t.Cleanup(s.Close)
	c := connect(t, s, 100, 30)
	id := openChat(t, c)
	c.Send(&api.Command{Op: api.OpZoom, Target: id})
	nextLayout(t, c, func(*api.Layout) bool { return true })
	c.Send(&api.Command{Op: api.OpNewWindow, Args: map[string]string{api.ArgKind: api.PaneChat}})
	nextLayout(t, c, func(l *api.Layout) bool { return len(l.Windows) == 2 })
	var windows, terminals, active int
	s.call(func(st *state) {
		sess := st.sessions["work"]
		snap := st.snapshot(sess)
		windows, active = len(snap.Windows), sess.active
		for _, w := range snap.Windows {
			terminals += len(w.Panes)
		}
		st.save(sess)
	})
	if windows != 1 || terminals != 1 {
		t.Fatalf("saved %d windows with %d panes, want 1 with 1", windows, terminals)
	}
	// The active window is the chat-only one the file leaves out; the
	// session must still load and restore.
	var saved store.Snapshot
	eventually(t, "the session file", func() bool {
		var err error
		saved, err = openStore(t, opts.SessionsDir).Load("work")
		return err == nil
	})
	if len(saved.Windows) != 1 || saved.Active != active || saved.Windows[0].Index == active {
		t.Fatalf("file has %d windows, active %d (session's %d)", len(saved.Windows), saved.Active, active)
	}
	var problem string
	s.call(func(st *state) { problem = st.saveProblem })
	if problem != "" {
		t.Fatalf("save problem: %s", problem)
	}
	var restored, restoredActive int
	s.call(func(st *state) {
		sess, err := st.restore(&saved, 100, 30)
		if err != nil {
			t.Errorf("restore: %v", err)
			return
		}
		restored, restoredActive = len(sess.windows), sess.active
		st.closeSession(sess)
	})
	if restored != 1 || restoredActive != saved.Windows[0].Index {
		t.Fatalf("restored %d windows, active %d", restored, restoredActive)
	}
}

func openStore(t *testing.T, dir string) *store.Store {
	t.Helper()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

// TestChatInputRefused: input for another agent, or an answer to no
// request, is refused, and the turn is not touched.
func TestChatInputRefused(t *testing.T) {
	s, _, _ := newChatServer(t)
	c := connect(t, s, 100, 30)
	id := openChat(t, c)
	agent := c.Chat(id).Agent
	for _, in := range []*api.ChatInput{
		{Pane: id, Agent: "someone-else", Kind: api.ChatInputInterrupt},
		{Pane: id, Agent: agent, Kind: api.ChatInputAnswer, ID: "nope", Decision: api.ChatDecisionYes},
		{Pane: id, Agent: agent, Kind: "dance"},
	} {
		c.Send(in)
		deadline := time.Now().Add(apitest.Wait)
		var got *api.Error
		for got == nil && time.Now().Before(deadline) {
			r, _ := c.Next(time.Second)
			got, _ = r.(*api.Error)
		}
		if got == nil {
			t.Fatalf("%+v was not refused", in)
		}
	}
}

// TestChatNotSetUp: without chat options, a chat pane is refused and the
// window is unchanged.
func TestChatNotSetUp(t *testing.T) {
	s := newTestServer(t)
	c := connect(t, s, 100, 30)
	c.Send(&api.Attach{Session: "work", Create: true})
	l := nextLayout(t, c, func(*api.Layout) bool { return true })
	c.Send(&api.Command{Op: api.OpSplit, Target: shownPanes(l)[0].ID, Args: map[string]string{api.ArgDir: api.DirRight, api.ArgKind: api.PaneChat}})
	for {
		m, err := c.Next(5 * time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if e, ok := m.(*api.Error); ok {
			if e.Code != api.CodeCommandFailed || !strings.Contains(e.Message, "not set up") {
				t.Fatalf("error %+v", e)
			}
			return
		}
	}
}

// TestSnapshotDoesNotShareTools: a snapshot sent to a client can be
// encoded on its writer goroutine while the owner goroutine applies a
// tool result. Run with -race.
func TestSnapshotDoesNotShareTools(t *testing.T) {
	snap := api.ChatSnapshot{Pane: 1, Agent: "a"}
	snap.Apply(&api.ChatEvent{Pane: 1, Kind: api.ChatEventToolCall, Tool: &api.ChatTool{ID: "t1", Name: "read"}})
	snap.Apply(&api.ChatEvent{Pane: 1, Kind: api.ChatEventApproval, Approval: &api.ChatApproval{ID: "t1", Tool: "read"}})
	out := fitSnapshot(snap)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 200 {
			if _, err := json.Marshal(out); err != nil {
				t.Error(err)
			}
		}
	}()
	for range 200 {
		snap.Apply(&api.ChatEvent{Pane: 1, Kind: api.ChatEventApproval, Approval: &api.ChatApproval{ID: "t1", Tool: "read", Diff: &api.ChatDiff{Path: "a"}}})
		snap.Apply(&api.ChatEvent{Pane: 1, Kind: api.ChatEventToolResult, Tool: &api.ChatTool{ID: "t1", Done: true, Result: "x"}})
	}
	<-done
	if out.Entries[0].Tool.Done {
		t.Fatal("the sent snapshot changed after it was taken")
	}
}

// TestChatQueueBounded: a running turn holds at most MaxChatQueue
// messages; the next one is refused, so the queue always fits a frame.
func TestChatQueueBounded(t *testing.T) {
	hold := make(chan struct{})
	r := reply("ok")
	r.Hold = hold
	s, _, _ := newChatServer(t, r)
	c := connect(t, s, 100, 30)
	id := openChat(t, c)
	say(c, id, "first")
	c.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Running })
	for i := range api.MaxChatQueue {
		say(c, id, fmt.Sprintf("then %d", i))
		c.WaitChat(id, func(s *api.ChatSnapshot) bool { return len(s.Queue) == i+1 })
	}
	say(c, id, "one too many")
	deadline := time.Now().Add(apitest.Wait)
	var got *api.Error
	for got == nil && time.Now().Before(deadline) {
		r, _ := c.Next(time.Second)
		got, _ = r.(*api.Error)
	}
	if got == nil {
		t.Fatal("a message past the queue limit was not refused")
	}
	close(hold)
}
