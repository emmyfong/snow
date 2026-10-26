package server

import (
	"encoding/json"
	"testing"

	"github.com/emmyfong/snow/internal/apitest"
	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providertest"
	"github.com/emmyfong/snow/internal/tool"
	"github.com/emmyfong/snow/pkg/api"
)

// askReply is a reply that asks "Which store?" with options A and B.
func askReply(id string) providertest.Reply {
	in, _ := json.Marshal(map[string]any{
		"question": "Which store?",
		"options":  []map[string]string{{"label": "A", "description": "the first"}, {"label": "B"}},
	})
	return providertest.Reply{Events: []provider.Event{
		&provider.Start{Model: "model-1"},
		&provider.ToolUse{ID: id, Name: "ask", Input: in},
		&provider.Stop{Reason: provider.StopToolUse},
	}}
}

// askingChat starts a server whose model asks a question, opens a chat in
// session "work", sends a message, and waits for the question.
func askingChat(t *testing.T, replies ...providertest.Reply) (*Server, *apitest.Client, int, *api.ChatSnapshot) {
	t.Helper()
	dir := t.TempDir()
	opts, _, _ := chatOptions(t, append([]providertest.Reply{askReply("q1")}, replies...)...)
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
	c.WaitChat(id, func(*api.ChatSnapshot) bool { return true })
	say(c, id, "pick a store")
	snap := c.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Question != nil && s.State == api.ChatStateApproval })
	return s, c, id, snap
}

// TestQuestionEndToEnd: the question reaches the client with its options,
// and the reply sent as a ChatInput reaches the model as the tool result.
func TestQuestionEndToEnd(t *testing.T) {
	_, c, id, snap := askingChat(t, reply("thanks"))
	q := snap.Question
	if q.ID != "q1" || q.Text != "Which store?" || len(q.Options) != 2 || q.Options[0].Description != "the first" || !snap.Running {
		t.Fatalf("question %+v", q)
	}
	if snap.Pending != nil {
		t.Fatalf("ask asked for approval: %+v", snap.Pending)
	}
	c.Send(&api.ChatInput{Pane: id, Agent: snap.Agent, Kind: api.ChatInputReply, ID: "q1", Picked: []string{"B"}})
	done := c.WaitChat(id, func(s *api.ChatSnapshot) bool { return lastReply(s) == "thanks" && idle(s) })
	if done.Question != nil {
		t.Fatalf("question left: %+v", done.Question)
	}
	var cell *api.ChatTool
	for _, e := range done.Entries {
		if e.Tool != nil {
			cell = e.Tool
		}
	}
	if cell == nil || cell.Name != "ask" || !cell.Done || cell.IsError || cell.Result != `The user picked "B".` {
		t.Fatalf("tool cell %+v", cell)
	}
}
