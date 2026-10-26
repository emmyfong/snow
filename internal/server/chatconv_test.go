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
	nextError(t, c)
}

// TestModeCommand: ask and auto change the mode of the pane now; a word
// that is neither is refused and the mode stays.
func TestModeCommand(t *testing.T) {
	for _, tt := range []struct {
		name, mode, want string
		refused          bool
	}{
		{"auto", api.ChatModeAuto, api.ChatModeAuto, false},
		{"ask", api.ChatModeAsk, api.ChatModeAsk, false},
		{"bad word", "yolo", api.ChatModeAsk, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, _, store := newChatServer(t)
			c := connect(t, s, 100, 30)
			id := openChat(t, c)
			chatInputOf(c, id, api.ChatInputMode, func(in *api.ChatInput) { in.Mode = api.ChatModeAuto })
			c.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Mode == api.ChatModeAuto })
			chatInputOf(c, id, api.ChatInputMode, func(in *api.ChatInput) { in.Mode = tt.mode })
			if tt.refused {
				nextError(t, c)
				if got := c.Chat(id).Mode; got != api.ChatModeAuto {
					t.Fatalf("a refused word changed the mode to %q", got)
				}
				return
			}
			c.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Mode == tt.want })
			eventually(t, "the mode record", func() bool {
				for _, r := range conversationRecords(t, store, c.Chat(id).Conversation) {
					if m, ok := r.(*convstore.Mode); ok && m.Mode == tt.want {
						return true
					}
				}
				return false
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
	nextError(t, c)
}
