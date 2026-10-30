package server

import (
	"testing"

	"github.com/emmyfong/snow/pkg/api"
)

// TestEmptyChatResume: a new session's empty chat lists a past conversation of
// its folder, and resuming it shows the transcript.
func TestEmptyChatResume(t *testing.T) {
	s, _, store := newChatServer(t)
	c := connect(t, s, 100, 30)
	id := openChat(t, c)
	own, err := store.Load(c.Chat(id).Conversation)
	if err != nil {
		t.Fatal(err)
	}
	past := saveConversation(t, store, own.Meta.Folder, pastExchange()...)

	c.Send(&api.ConversationListRequest{Pane: id})
	l := waitConversations(t, c, func(l *api.ConversationList) bool { return l.Pane == id })
	if len(l.Items) != 1 || l.Items[0].ID != past || l.Items[0].Open {
		t.Fatalf("list %+v, want only the past conversation, not the empty chat", l.Items)
	}
	chatInputOf(c, id, api.ChatInputResume, func(in *api.ChatInput) { in.Conversation = past })
	got := c.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Conversation == past })
	if len(got.Entries) == 0 {
		t.Fatalf("the resumed chat shows no transcript: %+v", got)
	}
}

// TestEmptyChatOpenElsewhere: the list marks a conversation that another pane
// shows as open, and resuming that row moves focus to the pane.
func TestEmptyChatOpenElsewhere(t *testing.T) {
	s, _, _ := newChatServer(t, reply("ok"))
	c := connect(t, s, 100, 30)
	first := openChat(t, c)
	say(c, first, "hello")
	c.WaitChat(first, func(s *api.ChatSnapshot) bool { return idle(s) && lastReply(s) == "ok" })
	shown := c.Chat(first).Conversation
	second := openSecondChat(t, c, first)

	c.Send(&api.ConversationListRequest{Pane: second})
	l := waitConversations(t, c, func(l *api.ConversationList) bool { return l.Pane == second })
	if len(l.Items) != 1 || l.Items[0].ID != shown || !l.Items[0].Open {
		t.Fatalf("list %+v, want the first pane's conversation marked open", l.Items)
	}
	chatInputOf(c, second, api.ChatInputResume, func(in *api.ChatInput) { in.Conversation = shown })
	nextLayout(t, c, func(l *api.Layout) bool {
		for _, p := range shownPanes(l) {
			if p.ID == first && p.Focused {
				return true
			}
		}
		return false
	})
}
