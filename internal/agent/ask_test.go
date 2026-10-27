package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providertest"
	"github.com/emmyfong/snow/internal/tool"
)

// askTool is the built-in ask tool.
func askTool(t *testing.T) tool.Tool {
	t.Helper()
	a, ok := tool.Builtin().Get("ask")
	if !ok {
		t.Fatal("ask is not built in")
	}
	return a
}

// askCall is a reply that asks "Which store?" with options A, B, and C.
func askCall(id string, multi bool) providertest.Reply {
	in, _ := json.Marshal(map[string]any{
		"question": "Which store?", "multi": multi,
		"options": []map[string]string{{"label": "A", "preview": "a code"}, {"label": "B"}, {"label": "C"}},
	})
	return providertest.Reply{Events: []provider.Event{
		&provider.ToolUse{ID: id, Name: "ask", Input: in}, &provider.Stop{Reason: provider.StopToolUse},
	}}
}

// askAndReply asks once, answers with r, and returns the result the
// model got.
func askAndReply(t *testing.T, multi bool, r QuestionReply) (*harness, *provider.ToolResult) {
	t.Helper()
	h := newHarness(t, []tool.Tool{askTool(t)}, askCall("q1", multi), text("thanks"))
	_ = h.conv.Send("pick a store")
	q := next[*Question](t, h)
	if q.ID != "q1" || q.Text != "Which store?" || q.Multi != multi || len(q.Options) != 3 || q.Options[0].Preview != "a code" {
		t.Fatalf("question %+v", q)
	}
	if got := h.conv.Question(); got == nil || got.ID != "q1" {
		t.Fatalf("Question() = %+v while it waits", got)
	}
	r.ID = "q1"
	if err := h.conv.Reply(r); err != nil {
		t.Fatal(err)
	}
	if a := next[*QuestionAnswered](t, h); a.ID != "q1" {
		t.Fatalf("answered %+v", a)
	}
	if e := next[*TurnEnded](t, h); e.Reason != EndDone {
		t.Fatalf("turn ended %+v", e)
	}
	if got := h.conv.Question(); got != nil {
		t.Fatalf("Question() = %+v after the answer", got)
	}
	reqs := h.fake.Requests()
	msgs := reqs[1].Messages
	return h, msgs[len(msgs)-1].Content[0].(*provider.ToolResult)
}

func TestAskSingle(t *testing.T) {
	h, res := askAndReply(t, false, QuestionReply{Picked: []string{"B"}})
	if res.IsError || res.Content != `The user picked "B".` {
		t.Fatalf("model got %+v", res)
	}
	var q *convstore.Question
	var a *convstore.Answer
	for _, r := range h.records(t) {
		switch r := r.(type) {
		case *convstore.Question:
			q = r
		case *convstore.Answer:
			a = r
		}
	}
	if q == nil || q.ToolUseID != "q1" || len(q.Options) != 3 || a == nil || a.ToolUseID != "q1" || len(a.Picked) != 1 || a.Picked[0] != "B" {
		t.Fatalf("saved question %+v, answer %+v", q, a)
	}
}

func TestAskMulti(t *testing.T) {
	_, res := askAndReply(t, true, QuestionReply{Picked: []string{"C", "A"}})
	if res.IsError || res.Content != `The user picked "A", "C".` {
		t.Fatalf("model got %+v", res)
	}
}

func TestAskOther(t *testing.T) {
	_, res := askAndReply(t, false, QuestionReply{Other: "Use SQLite"})
	if res.IsError || res.Content != "The user typed their own answer:\nUse SQLite" {
		t.Fatalf("model got %+v", res)
	}
}

// TestAskNeedsNoApprovalInAsk: ask changes nothing, so the ask mode lets
// it run without an approval request.
func TestAskNeedsNoApprovalInAsk(t *testing.T) {
	h := newHarness(t, []tool.Tool{askTool(t)}, askCall("q1", false), text("thanks"))
	if h.conv.Mode() != ModeAsk {
		t.Fatalf("mode %s", h.conv.Mode())
	}
	_ = h.conv.Send("pick")
	for {
		e := <-h.events
		switch b := e.Body.(type) {
		case *ApprovalRequest:
			t.Fatalf("approval request %+v", b)
		case *Question:
			if err := h.conv.Reply(QuestionReply{ID: b.ID, Picked: []string{"A"}}); err != nil {
				t.Fatal(err)
			}
		case *TurnEnded:
			if b.Reason != EndDone {
				t.Fatalf("turn ended %+v", b)
			}
			return
		}
	}
}

// TestAskInterrupt: Esc ends the turn and saves no answer. The question
// stays open, and an answer to it starts a turn that gives the model the
// question and the answer.
func TestAskInterrupt(t *testing.T) {
	h := newHarness(t, []tool.Tool{askTool(t)}, askCall("q1", false), text("on it"))
	_ = h.conv.Send("pick")
	next[*Question](t, h)
	if err := h.conv.Interrupt(); err != nil {
		t.Fatal(err)
	}
	if e := next[*TurnEnded](t, h); e.Reason != EndInterrupted {
		t.Fatalf("turn ended %+v", e)
	}
	h.waitIdle(t)
	for _, r := range h.records(t) {
		if _, ok := r.(*convstore.Answer); ok {
			t.Fatal("an interrupted question has an answer record")
		}
	}
	if q := h.conv.Question(); q == nil || q.ID != "q1" {
		t.Fatalf("Question() = %+v after the interrupt", q)
	}

	if err := h.conv.Reply(QuestionReply{ID: "q1", Picked: []string{"C"}}); err != nil {
		t.Fatal(err)
	}
	started := next[*TurnStarted](t, h)
	if want := "You asked: Which store?\nMy answer: C"; started.Text != want {
		t.Fatalf("turn text %q, want %q", started.Text, want)
	}
	next[*TurnEnded](t, h)
	recs := h.records(t)
	n := len(recs)
	// The message comes first: it carries the answer to the model, so a
	// crash after it loses nothing.
	if u, ok := recs[n-3].(*convstore.User); !ok || u.Text != started.Text {
		t.Fatalf("records %v", types(recs))
	}
	if a, ok := recs[n-2].(*convstore.Answer); !ok || a.ToolUseID != "q1" || a.Picked[0] != "C" {
		t.Fatalf("records %v", types(recs))
	}
	if q := h.conv.Question(); q != nil {
		t.Fatalf("Question() = %+v after the answer", q)
	}
}

// TestSendClosesQuestion: a new message moves on from an open question.
func TestSendClosesQuestion(t *testing.T) {
	h := newHarness(t, []tool.Tool{askTool(t)}, askCall("q1", false), text("ok"))
	_ = h.conv.Send("pick")
	next[*Question](t, h)
	_ = h.conv.Interrupt()
	next[*TurnEnded](t, h)
	h.waitIdle(t)
	_ = h.conv.Send("never mind")
	next[*TurnEnded](t, h)
	if q := h.conv.Question(); q != nil {
		t.Fatalf("Question() = %+v after a new message", q)
	}
	if err := h.conv.Reply(QuestionReply{ID: "q1", Picked: []string{"A"}}); !errors.Is(err, ErrNoQuestion) {
		t.Fatalf("late reply: %v, want ErrNoQuestion", err)
	}
}

func TestAskReplyRefused(t *testing.T) {
	h := newHarness(t, []tool.Tool{askTool(t)}, askCall("q1", false), text("ok"))
	_ = h.conv.Send("pick")
	next[*Question](t, h)
	tests := []struct {
		name string
		r    QuestionReply
		want error
	}{
		{"other ID", QuestionReply{ID: "q2", Picked: []string{"A"}}, ErrNoQuestion},
		{"unknown label", QuestionReply{ID: "q1", Picked: []string{"Z"}}, ErrInvalid},
		{"twice", QuestionReply{ID: "q1", Picked: []string{"A", "A"}}, ErrInvalid},
		{"two in single", QuestionReply{ID: "q1", Picked: []string{"A", "B"}}, ErrInvalid},
		{"nothing", QuestionReply{ID: "q1"}, ErrInvalid},
		{"both", QuestionReply{ID: "q1", Picked: []string{"A"}, Other: "x"}, ErrInvalid},
		{"blank other", QuestionReply{ID: "q1", Other: "  "}, ErrInvalid},
	}
	for _, tt := range tests {
		if err := h.conv.Reply(tt.r); !errors.Is(err, tt.want) {
			t.Errorf("%s: %v, want %v", tt.name, err, tt.want)
		}
	}
	if err := h.conv.Reply(QuestionReply{ID: "q1", Picked: []string{"A"}}); err != nil {
		t.Fatalf("the question did not survive the refused replies: %v", err)
	}
	next[*TurnEnded](t, h)
}

// TestLoadReportsOpenQuestion: a file that ends while a question waits
// loads with the question open and the turn interrupted. The answer
// starts a new turn that gives the model the question and the answer.
func TestLoadReportsOpenQuestion(t *testing.T) {
	h := newHarness(t, []tool.Tool{askTool(t)}, askCall("q1", false), text("using B"))
	_ = h.conv.Send("pick a store")
	next[*Question](t, h)
	loaded := closeAndLoad(t, h)
	if q := loaded.OpenQuestion; q == nil || q.ToolUseID != "q1" {
		t.Fatalf("loaded open question %+v", q)
	}

	c, again, err := ResumeLoaded(h.config(t), loaded)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	if again.MidTurn {
		t.Fatal("the resumed file still ends mid-turn")
	}
	if n := len(again.Records); again.Records[n-1].Type() != convstore.TypeInterruption {
		t.Fatalf("records %v; the turn that asked is not interrupted", types(again.Records))
	}
	q := c.Question()
	if q == nil || q.ID != "q1" || q.Text != "Which store?" || len(q.Options) != 3 || q.Options[0].Preview != "a code" {
		t.Fatalf("Question() = %+v after the resume", q)
	}
	if err := c.Reply(QuestionReply{ID: "q1", Picked: []string{"B"}}); err != nil {
		t.Fatal(err)
	}
	if s := next[*TurnStarted](t, h); s.Text != "You asked: Which store?\nMy answer: B" {
		t.Fatalf("turn text %q", s.Text)
	}
	next[*TurnEnded](t, h)
	reqs := h.fake.Requests()
	msgs := reqs[len(reqs)-1].Messages
	if last := msgs[len(msgs)-1].Content; last[len(last)-1].(*provider.Text).Text != "You asked: Which store?\nMy answer: B" {
		t.Fatalf("the model got %+v", last)
	}
	if c.Question() != nil {
		t.Fatal("the question is still open")
	}
}

// TestAnsweredQuestionNotOpen: a file with the answer loads with no open
// question.
func TestAnsweredQuestionNotOpen(t *testing.T) {
	h, _ := askAndReply(t, false, QuestionReply{Picked: []string{"A"}})
	loaded := closeAndLoad(t, h)
	if loaded.OpenQuestion != nil {
		t.Fatalf("open question %+v", loaded.OpenQuestion)
	}
	c, _, err := ResumeLoaded(h.config(t), loaded)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	if q := c.Question(); q != nil {
		t.Fatalf("Question() = %+v", q)
	}
	if err := c.Reply(QuestionReply{ID: "q1", Picked: []string{"A"}}); !errors.Is(err, ErrNoQuestion) {
		t.Fatalf("reply: %v, want ErrNoQuestion", err)
	}
}

// closeThen is a reply that calls "closer", which closes the conversation
// file, then the tool name with input in, so name's own saves fail.
func closeThen(name string, in json.RawMessage) providertest.Reply {
	return providertest.Reply{Events: []provider.Event{
		&provider.ToolUse{ID: "c1", Name: "closer", Input: json.RawMessage(`{}`)},
		&provider.ToolUse{ID: "x1", Name: name, Input: in},
		&provider.Stop{Reason: provider.StopToolUse},
	}}
}

// closer returns a tool that closes h's conversation file when it runs.
func closer(h **harness) fnTool {
	return fnTool{tool.Def{Name: "closer"}, func(context.Context) (string, error) {
		return "closed", (*h).conv.w.Close()
	}}
}

// TestSaveFailures: a question, an answer, or a task list that cannot be
// saved is never shown, and the turn fails with a save error.
func TestSaveFailures(t *testing.T) {
	askIn, _ := json.Marshal(map[string]any{"question": "Which?", "options": []map[string]string{{"label": "A"}, {"label": "B"}}})
	tests := []struct {
		name string
		in   json.RawMessage
	}{
		{"ask", askIn},
		{"tasks", json.RawMessage(`{"tasks":[{"text":"a","state":"pending"}]}`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var h *harness
			h = newHarness(t, []tool.Tool{askTool(t), tasksTool(t), closer(&h)}, closeThen(tt.name, tt.in))
			_ = h.conv.Send("go")
			for {
				e := <-h.events
				switch b := e.Body.(type) {
				case *Question, *TasksChanged:
					t.Fatalf("%T shown although it was not saved", b)
				case *TurnEnded:
					if b.Reason != EndFailed || b.Failure == nil || b.Failure.Kind != FailSave {
						t.Fatalf("turn ended %+v", b)
					}
					if q := h.conv.Question(); q != nil {
						t.Fatalf("Question() = %+v", q)
					}
					return
				}
			}
		})
	}

	t.Run("answer turn", func(t *testing.T) {
		h := newHarness(t, []tool.Tool{askTool(t)}, askCall("q1", false), text("never"))
		_ = h.conv.Send("pick")
		next[*Question](t, h)
		_ = h.conv.Interrupt()
		next[*TurnEnded](t, h)
		h.waitIdle(t)
		if err := h.conv.w.Close(); err != nil {
			t.Fatal(err)
		}
		if err := h.conv.Reply(QuestionReply{ID: "q1", Picked: []string{"A"}}); err != nil {
			t.Fatal(err)
		}
		if e := next[*TurnEnded](t, h); e.Reason != EndFailed || e.Failure == nil || e.Failure.Kind != FailSave {
			t.Fatalf("turn ended %+v", e)
		}
		if n := len(h.fake.Requests()); n != 1 {
			t.Fatalf("the provider got %d requests; the unsaved answer must not reach it", n)
		}
	})
}

// TestRetryRefusedWhileQuestionOpen: a retry sends no message after an
// interrupted question, so it would leave the question open in the file
// while the chat moved on. The user answers or sends a message instead.
func TestRetryRefusedWhileQuestionOpen(t *testing.T) {
	h := newHarness(t, []tool.Tool{askTool(t)}, askCall("q1", false), text("ok"))
	_ = h.conv.Send("pick")
	next[*Question](t, h)
	_ = h.conv.Interrupt()
	next[*TurnEnded](t, h)
	h.waitIdle(t)
	if err := h.conv.Retry(); !errors.Is(err, ErrBusy) {
		t.Fatalf("Retry: %v, want ErrBusy", err)
	}
	if q := h.conv.Question(); q == nil {
		t.Fatal("the refused retry closed the question")
	}
}
