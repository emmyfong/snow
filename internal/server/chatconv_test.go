package server

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/pkg/api"
)

// chatInputOf sends a ChatInput of kind to chat pane id and lets set fill
// in the rest.
func chatInputOf(c *apitest.Client, id int, kind string, set func(*api.ChatInput)) {
	in := &api.ChatInput{Pane: id, Agent: c.Chat(id).Agent, Kind: kind}
	if set != nil {
		set(in)
	}
	c.Send(in)
}

// wantRefused waits for an api.Error and checks it is a refusal of the
// input, not a crash.
func wantRefused(t *testing.T, c *apitest.Client) {
	t.Helper()
	if e := nextError(t, c); e.Code != api.CodeBadCommand && e.Code != api.CodeCommandFailed {
		t.Fatalf("error %+v is not a refusal", e)
	}
}

// conversationRecords loads the records of conversation id, without meta.
func conversationRecords(t *testing.T, store *convstore.Store, id string) []convstore.Record {
	t.Helper()
	c, err := store.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	return c.Records
}

// TestRenameAppendsRecord: /rename changes the title everyone sees and adds
// one record; every earlier byte of the file stays as it was.
func TestRenameAppendsRecord(t *testing.T) {
	s, _, store := newChatServer(t, reply("ok"))
	c := connect(t, s, 100, 30)
	id := openChat(t, c)
	say(c, id, "hi")
	c.WaitChat(id, func(s *api.ChatSnapshot) bool { return idle(s) && lastReply(s) == "ok" })
	conv := c.Chat(id).Conversation
	path, err := store.Path(conv)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	chatInputOf(c, id, api.ChatInputRename, func(in *api.ChatInput) { in.Text = "Fix the rename bug" })
	c.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Title == "Fix the rename bug" })
	eventually(t, "the record to be written", func() bool {
		after, _ := os.ReadFile(path)
		return len(after) > len(before)
	})
	after, _ := os.ReadFile(path)
	if !bytes.HasPrefix(after, before) {
		t.Fatal("a rename changed earlier lines")
	}
	recs := conversationRecords(t, store, conv)
	if r, ok := recs[len(recs)-1].(*convstore.Rename); !ok || r.Title != "Fix the rename bug" {
		t.Fatalf("last record %#v", recs[len(recs)-1])
	}
	chatInputOf(c, id, api.ChatInputRename, func(in *api.ChatInput) { in.Text = "   " })
	wantRefused(t, c)
}

// modeRecords counts the mode records of conversation id and returns the
// last one's mode.
func modeRecords(t *testing.T, store *convstore.Store, id string) (n int, last string) {
	t.Helper()
	for _, r := range conversationRecords(t, store, id) {
		if m, ok := r.(*convstore.Mode); ok {
			n, last = n+1, m.Mode
		}
	}
	return n, last
}

// TestModeCommand: ask and auto change the mode of the pane now, from the
// other mode; a word that is neither is refused, and nothing is saved.
func TestModeCommand(t *testing.T) {
	for _, tt := range []struct {
		name, start, mode, want string
		refused                 bool
	}{
		{name: "to auto", start: api.ChatModeAsk, mode: api.ChatModeAuto, want: api.ChatModeAuto},
		{name: "to ask", start: api.ChatModeAuto, mode: api.ChatModeAsk, want: api.ChatModeAsk},
		{name: "bad word", start: api.ChatModeAuto, mode: "yolo", want: api.ChatModeAuto, refused: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, _, store := newChatServer(t)
			c := connect(t, s, 100, 30)
			id := openChat(t, c)
			conv := c.Chat(id).Conversation
			if tt.start == api.ChatModeAuto {
				chatInputOf(c, id, api.ChatInputMode, func(in *api.ChatInput) { in.Mode = api.ChatModeAuto })
			}
			c.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Mode == tt.start })
			eventually(t, "the start mode to be saved", func() bool {
				n, _ := modeRecords(t, store, conv)
				return (tt.start == api.ChatModeAuto) == (n == 1)
			})
			before, _ := modeRecords(t, store, conv)

			chatInputOf(c, id, api.ChatInputMode, func(in *api.ChatInput) { in.Mode = tt.mode })
			if tt.refused {
				wantRefused(t, c)
				// An input after the refused one is handled after it, so once
				// its title shows, a write for the bad word would be there.
				chatInputOf(c, id, api.ChatInputRename, func(in *api.ChatInput) { in.Text = "after" })
				c.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Title == "after" })
				if got := c.Chat(id).Mode; got != tt.want {
					t.Fatalf("a refused word changed the mode to %q", got)
				}
				if n, _ := modeRecords(t, store, conv); n != before {
					t.Fatalf("a refused word wrote %d mode records", n-before)
				}
				return
			}
			c.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Mode == tt.want })
			eventually(t, "the mode record", func() bool {
				n, last := modeRecords(t, store, conv)
				return n == before+1 && last == tt.want
			})
		})
	}
}

// TestEffortCommand: a level the model takes is saved as a switch record
// and shows in the pane; a level it does not take is refused with the
// levels it does take, and nothing is written.
func TestEffortCommand(t *testing.T) {
	opts, fake, store := chatOptions(t)
	fake.SetModels([]provider.Model{
		{ID: "model-1", Efforts: []provider.Effort{provider.EffortMedium, provider.EffortHigh}},
		{ID: "plain"},
	}, nil)
	s := New(opts)
	t.Cleanup(s.Close)
	c := connect(t, s, 100, 30)
	id := openChat(t, c)
	conv := c.Chat(id).Conversation

	chatInputOf(c, id, api.ChatInputEffort, func(in *api.ChatInput) { in.Effort = "high" })
	c.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Effort == "high" })
	eventually(t, "the switch record", func() bool {
		for _, r := range conversationRecords(t, store, conv) {
			if sw, ok := r.(*convstore.ModelSwitch); ok && sw.Model == chatModel && sw.Effort == "high" {
				return true
			}
		}
		return false
	})

	chatInputOf(c, id, api.ChatInputEffort, func(in *api.ChatInput) { in.Effort = "max" })
	e := nextError(t, c)
	if !strings.Contains(e.Message, "medium") || !strings.Contains(e.Message, "high") || strings.Contains(e.Message, "low") {
		t.Fatalf("refusal %q does not list the levels the model takes", e.Message)
	}
	if got := c.Chat(id).Effort; got != "high" {
		t.Fatalf("a refused level changed the effort to %q", got)
	}

	chatInputOf(c, id, api.ChatInputModel, func(in *api.ChatInput) { in.Model = "fake/plain" })
	c.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Model == "fake/plain" })
	chatInputOf(c, id, api.ChatInputEffort, func(in *api.ChatInput) { in.Effort = "low" })
	if e := nextError(t, c); !strings.Contains(e.Message, "no effort") {
		t.Fatalf("refusal %q", e.Message)
	}
	chatInputOf(c, id, api.ChatInputEffort, nil) // the provider's own choice
	c.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Effort == "" && s.Model == "fake/plain" })
}

// waitConversations waits for a ConversationList that ok accepts.
func waitConversations(t *testing.T, c *apitest.Client, ok func(*api.ConversationList) bool) *api.ConversationList {
	t.Helper()
	deadline := time.Now().Add(apitest.Wait)
	for time.Now().Before(deadline) {
		m, _ := c.Next(100 * time.Millisecond)
		if l, isList := m.(*api.ConversationList); isList && ok(l) {
			return l
		}
	}
	t.Fatal("no matching ConversationList")
	return nil
}

// TestListConversations: the list holds the pane's folder, newest first,
// with the title, the turns, and which ones a pane shows; All adds the
// other folders.
func TestListConversations(t *testing.T) {
	s, _, store := newChatServer(t, reply("ok"), reply("ok"))
	c := connect(t, s, 100, 30)
	first := openChat(t, c)
	say(c, first, "explain the rename command")
	c.WaitChat(first, func(s *api.ChatSnapshot) bool { return idle(s) && lastReply(s) == "ok" })
	own := c.Chat(first).Conversation

	entries, err := store.ListAll()
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries %v, %v", entries, err)
	}
	w, err := store.Create(convstore.Meta{Folder: "/elsewhere", Profile: entries[0].Profile, Model: chatModel, Created: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Append(&convstore.User{At: time.Now(), Text: "an older question"}); err != nil {
		t.Fatal(err)
	}
	other := w.ID()
	_ = w.Close()

	c.Send(&api.ConversationListRequest{Pane: first})
	l := waitConversations(t, c, func(l *api.ConversationList) bool { return l.Pane == first })
	if len(l.Items) != 1 || l.All {
		t.Fatalf("this folder: %+v", l)
	}
	if it := l.Items[0]; it.ID != own || !it.Open || it.Turns != 1 || !strings.Contains(it.Title, "rename") || it.Model != chatModel {
		t.Fatalf("item %+v", it)
	}

	c.Send(&api.ConversationListRequest{Pane: first, All: true})
	l = waitConversations(t, c, func(l *api.ConversationList) bool { return l.All })
	if len(l.Items) != 2 || l.Items[0].ID != other || l.Items[0].Open {
		t.Fatalf("all folders: %+v", l.Items)
	}

	c.Send(&api.ConversationListRequest{Pane: 999})
	wantRefused(t, c)
}

// openSecondChat splits the focused pane of the shown window with another
// chat and returns its ID once c has its snapshot.
func openSecondChat(t *testing.T, c *apitest.Client, from int) int {
	t.Helper()
	c.Send(&api.Command{Op: api.OpSplit, Target: from, Args: map[string]string{api.ArgDir: api.DirDown, api.ArgKind: api.PaneChat}})
	l := nextLayout(t, c, func(l *api.Layout) bool {
		n := 0
		for _, p := range shownPanes(l) {
			if p.Kind == api.PaneChat {
				n++
			}
		}
		return n == 2
	})
	for _, p := range shownPanes(l) {
		if p.Kind == api.PaneChat && p.ID != from {
			c.WaitChat(p.ID, func(*api.ChatSnapshot) bool { return true })
			return p.ID
		}
	}
	t.Fatal("no second chat pane")
	return 0
}

// TestNewCommand: /new puts a new conversation in the pane, with the same
// model, an empty transcript, and its own file. The old file does not
// change, and the old conversation's agent ID no longer works there.
func TestNewCommand(t *testing.T) {
	s, _, store := newChatServer(t, reply("ok"))
	c := connect(t, s, 100, 30)
	id := openChat(t, c)
	say(c, id, "hi")
	c.WaitChat(id, func(s *api.ChatSnapshot) bool { return idle(s) && lastReply(s) == "ok" })
	old := *c.Chat(id)
	oldPath, _ := store.Path(old.Conversation)
	before, _ := os.ReadFile(oldPath)

	chatInputOf(c, id, api.ChatInputNew, nil)
	got := c.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Conversation != old.Conversation })
	if got.Agent == old.Agent || got.Agent != got.Conversation || len(got.Entries) != 0 || got.Model != old.Model || got.Mode != api.ChatModeAsk || !idle(got) {
		t.Fatalf("new snapshot %+v", got)
	}
	entries, err := store.ListAll()
	if err != nil || len(entries) != 2 {
		t.Fatalf("%d conversation files, %v; want 2", len(entries), err)
	}
	if after, _ := os.ReadFile(oldPath); !bytes.Equal(after, before) {
		t.Fatal("/new changed the old conversation's file")
	}
	c.Send(&api.ChatInput{Pane: id, Agent: old.Agent, Kind: api.ChatInputInterrupt})
	wantRefused(t, c)
}

// TestNewAndResumeRefusedWhileRunning: while a turn runs, /new and /resume
// say to stop it first, and the pane keeps its conversation.
func TestNewAndResumeRefusedWhileRunning(t *testing.T) {
	hold := make(chan struct{})
	r := reply("ok")
	r.Hold = hold
	s, _, _ := newChatServer(t, r)
	c := connect(t, s, 100, 30)
	id := openChat(t, c)
	conv := c.Chat(id).Conversation
	say(c, id, "hi")
	c.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Running })
	for _, kind := range []string{api.ChatInputNew, api.ChatInputResume} {
		chatInputOf(c, id, kind, func(in *api.ChatInput) { in.Conversation = "someone" })
		if e := nextError(t, c); !strings.Contains(e.Message, "Esc") {
			t.Fatalf("%s: refusal %q", kind, e.Message)
		}
	}
	close(hold)
	got := c.WaitChat(id, func(s *api.ChatSnapshot) bool { return idle(s) })
	if got.Conversation != conv {
		t.Fatal("a refused command changed the conversation")
	}
}

// TestResumeLoadsConversation: /resume shows the past conversation's
// transcript and uses the model recorded in its file.
func TestResumeLoadsConversation(t *testing.T) {
	s, _, _ := newChatServer(t, reply("first answer"))
	c := connect(t, s, 100, 30)
	first := openChat(t, c)
	chatInputOf(c, first, api.ChatInputModel, func(in *api.ChatInput) { in.Model = "fake/model-2"; in.Effort = "high" })
	c.WaitChat(first, func(s *api.ChatSnapshot) bool { return s.Model == "fake/model-2" })
	chatInputOf(c, first, api.ChatInputRename, func(in *api.ChatInput) { in.Text = "Rename work" })
	chatInputOf(c, first, api.ChatInputMode, func(in *api.ChatInput) { in.Mode = api.ChatModeAuto })
	say(c, first, "explain the rename command")
	c.WaitChat(first, func(s *api.ChatSnapshot) bool {
		return idle(s) && lastReply(s) == "first answer" && s.Title == "Rename work" && s.Mode == api.ChatModeAuto
	})
	past := c.Chat(first).Conversation

	second := openSecondChat(t, c, first)
	c.Send(&api.Command{Op: api.OpClose, Target: first})
	nextLayout(t, c, func(l *api.Layout) bool {
		for _, p := range shownPanes(l) {
			if p.ID == first {
				return false
			}
		}
		return true
	})

	chatInputOf(c, second, api.ChatInputResume, func(in *api.ChatInput) { in.Conversation = past })
	got := c.WaitChat(second, func(s *api.ChatSnapshot) bool { return s.Conversation == past })
	if got.Model != "fake/model-2" || got.Effort != "high" || got.Title != "Rename work" || got.Mode != api.ChatModeAuto || !idle(got) {
		t.Fatalf("resumed snapshot %+v", got)
	}
	var user, answer bool
	for _, e := range got.Entries {
		user = user || (e.Kind == api.ChatEntryUser && e.Text == "explain the rename command")
		answer = answer || (e.Kind == api.ChatEntryAssistant && e.Text == "first answer")
	}
	if !user || !answer {
		t.Fatalf("the transcript lacks the exchange: %+v", got.Entries)
	}
}

// TestResumeOpenElsewhere: a conversation another pane shows moves focus
// there. This pane keeps its own conversation, and no second writer opens.
func TestResumeOpenElsewhere(t *testing.T) {
	s, _, store := newChatServer(t)
	c := connect(t, s, 100, 30)
	first := openChat(t, c)
	second := openSecondChat(t, c, first)
	shown := c.Chat(first).Conversation
	own := c.Chat(second).Conversation

	chatInputOf(c, second, api.ChatInputResume, func(in *api.ChatInput) { in.Conversation = shown })
	l := nextLayout(t, c, func(l *api.Layout) bool {
		for _, p := range shownPanes(l) {
			if p.ID == first && p.Focused {
				return true
			}
		}
		return false
	})
	if l == nil {
		t.Fatal("focus did not move")
	}
	if got := c.Chat(second).Conversation; got != own {
		t.Fatalf("the pane changed its conversation to %s", got)
	}
	if entries, _ := store.ListAll(); len(entries) != 2 {
		t.Fatalf("%d conversation files, want 2", len(entries))
	}
	// A bad id and a missing file are refused.
	for _, bad := range []string{"../escape", "no-such-conversation"} {
		chatInputOf(c, second, api.ChatInputResume, func(in *api.ChatInput) { in.Conversation = bad })
		wantRefused(t, c)
	}
}

// TestResumeNeverChangesTheModel: a conversation whose model no provider
// offers keeps that model on resume; Snow does not pick another.
func TestResumeNeverChangesTheModel(t *testing.T) {
	s, _, store := newChatServer(t)
	c := connect(t, s, 100, 30)
	id := openChat(t, c)
	w, err := store.Create(convstore.Meta{Folder: folderOf(t, store, c, id), Model: "retired/model-0", Effort: "high", Created: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	past := w.ID()
	_ = w.Close()

	chatInputOf(c, id, api.ChatInputResume, func(in *api.ChatInput) { in.Conversation = past })
	got := c.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Conversation == past })
	if got.Model != "retired/model-0" || got.Effort != "high" {
		t.Fatalf("model %q, effort %q after the resume", got.Model, got.Effort)
	}
}

// folderOf returns the folder the conversation of pane id works in.
func folderOf(t *testing.T, store *convstore.Store, c *apitest.Client, id int) string {
	t.Helper()
	loaded, err := store.Load(c.Chat(id).Conversation)
	if err != nil {
		t.Fatal(err)
	}
	return loaded.Meta.Folder
}

// TestDamagedConversationIsListedAndRefused: a file that cannot be read
// shows in the list with its reason, and a resume of it is refused and
// leaves the pane alone.
func TestDamagedConversationIsListedAndRefused(t *testing.T) {
	s, _, store := newChatServer(t)
	c := connect(t, s, 100, 30)
	id := openChat(t, c)
	own := c.Chat(id).Conversation
	const bad = "20260101-000000-deadbeef"
	path, err := store.Path(bad)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("this is not json\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	c.Send(&api.ConversationListRequest{Pane: id, All: true})
	l := waitConversations(t, c, func(l *api.ConversationList) bool { return l.All })
	found := false
	for _, it := range l.Items {
		if it.ID == bad {
			found = true
			if it.Err == "" {
				t.Fatal("the damaged file is listed without a reason")
			}
		}
	}
	if !found {
		t.Fatalf("the damaged file is not listed: %+v", l.Items)
	}

	chatInputOf(c, id, api.ChatInputResume, func(in *api.ChatInput) { in.Conversation = bad })
	wantRefused(t, c)
	if got := c.Chat(id).Conversation; got != own {
		t.Fatal("the pane changed its conversation")
	}
}
