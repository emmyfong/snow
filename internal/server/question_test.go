package server

import (
	"encoding/json"
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

// chatInFolder starts a server with replies and the built-in tools, and
// opens a chat in folder dir in session "work".
func chatInFolder(t *testing.T, dir string, replies ...providertest.Reply) (*Server, *convstore.Store, *apitest.Client, int) {
	t.Helper()
	opts, _, store := chatOptions(t, replies...)
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
	return s, store, c, id
}

// askingChat starts a server whose model asks a question, opens a chat in
// session "work", sends a message, and waits for the question.
func askingChat(t *testing.T, replies ...providertest.Reply) (*Server, *apitest.Client, int, *api.ChatSnapshot) {
	t.Helper()
	s, _, c, id := chatInFolder(t, t.TempDir(), append([]providertest.Reply{askReply("q1")}, replies...)...)
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

// askCell returns the ask tool's cell in s.
func askCell(s *api.ChatSnapshot) *api.ChatTool {
	for _, e := range s.Entries {
		if e.Tool != nil && e.Tool.Name == "ask" {
			return e.Tool
		}
	}
	return nil
}

// TestQuestionSurvivesDetach: the question waits while no client is
// attached, a client that attaches again gets it, and its answer ends the
// wait.
func TestQuestionSurvivesDetach(t *testing.T) {
	s, c, id, _ := askingChat(t, reply("thanks"))
	c.Send(&api.Detach{})
	c2 := connect(t, s, 100, 30)
	c2.Send(&api.Attach{Session: "work"})
	again := c2.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Question != nil })
	if again.Question.ID != "q1" || !again.Running || again.State != api.ChatStateApproval || len(again.Question.Options) != 2 {
		t.Fatalf("after attach: %+v", again)
	}
	c2.Send(&api.ChatInput{Pane: id, Agent: again.Agent, Kind: api.ChatInputReply, ID: "q1", Text: "neither"})
	done := c2.WaitChat(id, func(s *api.ChatSnapshot) bool { return lastReply(s) == "thanks" && idle(s) })
	if done.Question != nil {
		t.Fatalf("question left: %+v", done.Question)
	}
	if cell := askCell(done); cell == nil || cell.Result != "The user typed their own answer:\nneither" {
		t.Fatalf("ask cell %+v", cell)
	}
}

// TestSecondClientSeesQuestion: two attached clients both get the
// question. The first answer wins; the second is refused.
func TestSecondClientSeesQuestion(t *testing.T) {
	s, c, id, snap := askingChat(t, reply("thanks"))
	c2 := connect(t, s, 100, 30)
	c2.Send(&api.Attach{Session: "work"})
	if q := c2.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Question != nil }).Question; q.ID != "q1" {
		t.Fatalf("second client question %+v", q)
	}
	c.Send(&api.ChatInput{Pane: id, Agent: snap.Agent, Kind: api.ChatInputReply, ID: "q1", Picked: []string{"B"}})
	c.WaitChat(id, func(s *api.ChatSnapshot) bool { return lastReply(s) == "thanks" && idle(s) })
	c2.Send(&api.ChatInput{Pane: id, Agent: snap.Agent, Kind: api.ChatInputReply, ID: "q1", Picked: []string{"A"}})
	if e := nextError(t, c2); e.Code != api.CodeCommandFailed || !strings.Contains(e.Message, "no question") {
		t.Fatalf("late reply: %+v", e)
	}
	done := c2.WaitChat(id, func(s *api.ChatSnapshot) bool { return idle(s) && s.Question == nil })
	if cell := askCell(done); cell == nil || cell.Result != `The user picked "B".` {
		t.Fatalf("ask cell %+v", cell)
	}
}

// TestResumeShowsOpenQuestion: a conversation that Snow stopped while a
// question waited opens with the question and the turn interrupted. The
// answer starts a new turn.
func TestResumeShowsOpenQuestion(t *testing.T) {
	dir := t.TempDir()
	_, store, c, id := chatInFolder(t, dir, reply("using B"))
	at := time.Now()
	in := json.RawMessage(`{"question":"Which store?","options":[{"label":"A"},{"label":"B"}]}`)
	past := saveConversation(t, store, dir,
		&convstore.User{At: at, Text: "pick a store"},
		&convstore.Assistant{At: at, Stop: convstore.StopToolUse, Blocks: []convstore.Block{{Kind: convstore.BlockToolUse, ID: "q1", Name: "ask", Input: in}}},
		&convstore.Question{At: at, ToolUseID: "q1", Text: "Which store?", Options: []convstore.QuestionOption{{Label: "A"}, {Label: "B"}}},
	)
	chatInputOf(c, id, api.ChatInputResume, func(in *api.ChatInput) { in.Conversation = past })
	got := c.WaitChat(id, func(s *api.ChatSnapshot) bool { return s.Conversation == past })
	if got.Question == nil || got.Question.ID != "q1" || got.Running {
		t.Fatalf("resumed: question %+v, running %v", got.Question, got.Running)
	}
	if last := got.Entries[len(got.Entries)-1]; last.Kind != api.ChatEntryInterrupted {
		t.Fatalf("last entry %+v, want the turn interrupted", last)
	}
	c.Send(&api.ChatInput{Pane: id, Agent: got.Agent, Kind: api.ChatInputReply, ID: "q1", Picked: []string{"B"}})
	done := c.WaitChat(id, func(s *api.ChatSnapshot) bool { return lastReply(s) == "using B" && idle(s) })
	if done.Question != nil {
		t.Fatalf("question left: %+v", done.Question)
	}
	var asked string
	for _, e := range done.Entries {
		if e.Kind == api.ChatEntryUser {
			asked = e.Text
		}
	}
	if asked != "You asked: Which store?\nMy answer: B" {
		t.Fatalf("the answer turn says %q", asked)
	}
}
