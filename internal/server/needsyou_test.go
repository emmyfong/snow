package server

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providertest"
	"github.com/emmyfong/snow/internal/tool"
	"github.com/emmyfong/snow/pkg/api"
)

// needsYouIn returns the needs-you count of session name in l.
func needsYouIn(l *api.SessionList, name string) int {
	for _, s := range l.Sessions {
		if s.Name == name {
			return s.NeedsYou
		}
	}
	return -1
}

// TestSessionListNeedsYou: a needs-you hook message shows in the next
// session list, for the home page, with count 1.
func TestSessionListNeedsYou(t *testing.T) {
	s := newTestServer(t)
	_, first := attachNoPrompt(t, s)
	home := connect(t, s, 80, 24)
	hook(t, s, first, api.AgentNeedsYou, time.Now())
	nextList(t, home, func(l *api.SessionList) bool { return needsYouIn(l, "work") == 1 })
}

// TestNeedsYouClears: a working hook message clears the mark and sends a
// new list; so does closing the waiting pane.
func TestNeedsYouClears(t *testing.T) {
	s := newTestServer(t)
	c, first := attachNoPrompt(t, s)
	home := connect(t, s, 80, 24)
	now := time.Now()
	hook(t, s, first, api.AgentNeedsYou, now)
	nextList(t, home, func(l *api.SessionList) bool { return needsYouIn(l, "work") == 1 })
	hook(t, s, first, api.AgentWorking, now.Add(time.Millisecond))
	nextList(t, home, func(l *api.SessionList) bool { return needsYouIn(l, "work") == 0 })

	c.Send(split(first, api.DirRight))
	l := nextLayout(t, c, func(l *api.Layout) bool { return len(shownPanes(l)) == 2 })
	second := shownPanes(l)[1].ID
	hook(t, s, second, api.AgentNeedsYou, now.Add(2*time.Millisecond))
	hook(t, s, first, api.AgentNeedsYou, now.Add(3*time.Millisecond))
	nextList(t, home, func(l *api.SessionList) bool { return needsYouIn(l, "work") == 2 })
	c.Send(&api.Command{Op: api.OpClose, Target: second})
	nextList(t, home, func(l *api.SessionList) bool { return needsYouIn(l, "work") == 1 })
}

// TestNeedsYouChat: a Snow chat pane that waits for an approval counts
// too, and stops counting once answered.
func TestNeedsYouChat(t *testing.T) {
	dir := t.TempDir()
	opts, _, _ := chatOptions(t,
		providertest.Reply{Events: []provider.Event{
			&provider.Start{Model: "model-1"},
			&provider.ToolUse{ID: "t1", Name: "write", Input: json.RawMessage(`{"path":"out.txt","content":"hi\n"}`)},
			&provider.Stop{Reason: provider.StopToolUse},
		}},
		reply("written"),
	)
	opts.Chat.Tools = tool.Builtin()
	s := New(opts)
	t.Cleanup(s.Close)
	c := connect(t, s, 100, 30)
	c.Send(&api.Attach{Session: "work", Create: true, Dir: dir})
	l := nextLayout(t, c, func(*api.Layout) bool { return true })
	c.Send(&api.Command{Op: api.OpSplit, Target: shownPanes(l)[0].ID, Args: map[string]string{
		api.ArgDir: api.DirRight, api.ArgKind: api.PaneChat, api.ArgFolder: dir,
	}})
	id := chatIn(nextLayout(t, c, func(l *api.Layout) bool { return chatIn(l) != 0 }))
	snap := c.WaitChat(id, func(*api.ChatSnapshot) bool { return true })
	home := connect(t, s, 80, 24)
	say(c, id, "write it")
	nextList(t, home, func(l *api.SessionList) bool { return needsYouIn(l, "work") == 1 })
	c.Send(&api.ChatInput{Pane: id, Agent: snap.Agent, Kind: api.ChatInputAnswer, ID: "t1", Decision: api.ChatDecisionYes})
	nextList(t, home, func(l *api.SessionList) bool { return needsYouIn(l, "work") == 0 })
}

// TestNeedsYouChatQuestion: a Snow chat pane that asks the user a question
// counts as needing you until the answer.
func TestNeedsYouChatQuestion(t *testing.T) {
	s, c, id, snap := askingChat(t, reply("thanks"))
	home := connect(t, s, 80, 24)
	// The list a new client gets counts the open question.
	c.Send(&api.Detach{})
	nextList(t, c, func(l *api.SessionList) bool { return needsYouIn(l, "work") == 1 })
	c.Send(&api.Attach{Session: "work"})
	nextLayout(t, c, func(*api.Layout) bool { return true })
	c.Send(&api.ChatInput{Pane: id, Agent: snap.Agent, Kind: api.ChatInputReply, ID: "q1", Picked: []string{"A"}})
	nextList(t, home, func(l *api.SessionList) bool { return needsYouIn(l, "work") == 0 })
}

// TestChatWorkingSendsFewLayouts: a running turn shows as working, and the
// streamed text, which changes no state, sends no layout of its own.
func TestChatWorkingSendsFewLayouts(t *testing.T) {
	deltas := make([]string, 12)
	for i := range deltas {
		deltas[i] = "word "
	}
	s, _, _ := newChatServer(t, reply(deltas...))
	c := connect(t, s, 100, 30)
	id := openChat(t, c)
	say(c, id, "go")
	layouts, working := 0, false
	deadline := time.Now().Add(apitest.Wait)
	for time.Now().Before(deadline) {
		m, _ := c.Next(time.Second)
		if l, ok := m.(*api.Layout); ok {
			layouts++
			if a, _ := paneAgent(l, id); a == api.AgentWorking {
				working = true
			}
		}
		if snap := c.Chat(id); snap != nil && lastReply(snap) == strings.Repeat("word ", 12) && idle(snap) {
			break
		}
	}
	if !working {
		t.Fatal("no layout showed the chat working")
	}
	if layouts > 4 {
		t.Fatalf("%d layouts for one turn of 12 text deltas, want one per state change", layouts)
	}
}
