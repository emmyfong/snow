package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providertest"
	"github.com/emmyfong/snow/internal/tool"
)

func TestApprovalTable(t *testing.T) {
	change := tool.Def{Name: "edit", Changes: true}
	look := tool.Def{Name: "read"}
	out := []string{"../x"}
	tests := []struct {
		name string
		c    approvalCase
		want verdict
	}{
		{"read in ask", approvalCase{Tool: look, Mode: ModeAsk}, allow},
		{"change in ask", approvalCase{Tool: change, Mode: ModeAsk}, ask},
		{"change in ask, granted", approvalCase{Tool: change, Mode: ModeAsk, Granted: true}, allow},
		{"change in auto", approvalCase{Tool: change, Mode: ModeAuto}, allow},
		{"read outside in ask", approvalCase{Tool: look, Mode: ModeAsk, Outside: out}, ask},
		{"read outside in auto", approvalCase{Tool: look, Mode: ModeAuto, Outside: out}, ask},
		{"change outside, granted, auto", approvalCase{Tool: change, Mode: ModeAuto, Granted: true, Outside: out}, ask},
	}
	for _, tt := range tests {
		if got := decide(tt.c); got != tt.want {
			t.Errorf("%s: %v, want %v", tt.name, got, tt.want)
		}
	}
}

// changer is a tool that changes things; it counts its runs on ran.
func changer(ran chan<- struct{}) fnTool {
	return fnTool{tool.Def{Name: "change", Changes: true}, func(context.Context) (string, error) {
		ran <- struct{}{}
		return "changed", nil
	}}
}

// pathCall is a reply that calls read on path.
func pathCall(id, path string) providertest.Reply {
	in, _ := json.Marshal(map[string]string{"path": path})
	return providertest.Reply{Events: []provider.Event{
		&provider.ToolUse{ID: id, Name: "read", Input: in}, &provider.Stop{Reason: provider.StopToolUse},
	}}
}

func TestDenyReasonReachesModel(t *testing.T) {
	ran := make(chan struct{}, 10)
	h := newHarness(t, []tool.Tool{changer(ran)}, call("t1", "change"), text("ok"))
	_ = h.conv.Send("go")
	req := next[*ApprovalRequest](t, h)
	if req.ID != "t1" || req.Tool != "change" {
		t.Fatalf("request %+v", req)
	}
	if err := h.conv.Answer(Answer{ID: "t1", Decision: DecisionNo, Reason: "not on main"}); err != nil {
		t.Fatal(err)
	}
	if r := next[*ToolResult](t, h); !r.IsError || r.Content != "denied: not on main" {
		t.Fatalf("result %+v", r)
	}
	next[*TurnEnded](t, h)
	if len(ran) != 0 {
		t.Fatal("the tool ran")
	}
	reqs := h.fake.Requests()
	last := reqs[1].Messages[len(reqs[1].Messages)-1].Content[0].(*provider.ToolResult)
	if last.Content != "denied: not on main" || !last.IsError {
		t.Fatalf("model got %+v", last)
	}
	var saved *convstore.Approval
	for _, r := range h.records(t) {
		if a, ok := r.(*convstore.Approval); ok {
			saved = a
		}
	}
	if saved == nil || saved.Decision != convstore.DecisionNo || saved.Reason != "not on main" || saved.ToolUseID != "t1" {
		t.Fatalf("approval record %+v", saved)
	}
}

// TestGrantLastsOneConversation: "yes for this conversation" stops the
// questions for that tool, and a resume asks again.
func TestGrantLastsOneConversation(t *testing.T) {
	ran := make(chan struct{}, 10)
	h := newHarness(t, []tool.Tool{changer(ran)}, call("t1", "change"), call("t2", "change"), text("ok"), call("t3", "change"), text("ok"))
	_ = h.conv.Send("go")
	next[*ApprovalRequest](t, h)
	if err := h.conv.Answer(Answer{ID: "t1", Decision: DecisionAlways}); err != nil {
		t.Fatal(err)
	}
	if end := next[*TurnEnded](t, h); end.Reason != EndDone {
		t.Fatalf("turn ended %+v", end)
	}
	if len(ran) != 2 {
		t.Fatalf("ran %d times", len(ran))
	}
	id := h.conv.ID()
	_ = h.conv.Close()
	c, _, err := Resume(h.config(t), id)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	_ = c.Send("again")
	if req := next[*ApprovalRequest](t, h); req.ID != "t3" {
		t.Fatalf("request %+v", req)
	}
	_ = c.Answer(Answer{ID: "t3", Decision: DecisionYes})
	next[*TurnEnded](t, h)
}

// TestPendingApprovalSurvivesDetach: nothing but an answer ends the wait,
// and Pending gives the request to a client that comes back.
func TestPendingApprovalSurvivesDetach(t *testing.T) {
	ran := make(chan struct{}, 10)
	h := newHarness(t, []tool.Tool{changer(ran)}, call("t1", "change"), text("ok"))
	_ = h.conv.Send("go")
	next[*ApprovalRequest](t, h)
	time.Sleep(200 * time.Millisecond) // no event may end the wait meanwhile
	if p := h.conv.Pending(); p == nil || p.ID != "t1" {
		t.Fatalf("pending %+v", p)
	}
	if len(ran) != 0 {
		t.Fatal("the tool ran without an answer")
	}
	if err := h.conv.Answer(Answer{ID: "other", Decision: DecisionYes}); !errors.Is(err, ErrNoApproval) {
		t.Fatalf("wrong ID: %v", err)
	}
	_ = h.conv.Answer(Answer{ID: "t1", Decision: DecisionYes})
	next[*TurnEnded](t, h)
	if len(ran) != 1 || h.conv.Pending() != nil {
		t.Fatalf("ran %d, pending %+v", len(ran), h.conv.Pending())
	}
}

func TestInterruptEndsApprovalWait(t *testing.T) {
	ran := make(chan struct{}, 10)
	h := newHarness(t, []tool.Tool{changer(ran)}, call("t1", "change"))
	_ = h.conv.Send("go")
	next[*ApprovalRequest](t, h)
	_ = h.conv.Interrupt()
	if end := next[*TurnEnded](t, h); end.Reason != EndInterrupted {
		t.Fatalf("turn ended %+v", end)
	}
	if h.conv.Pending() != nil || len(ran) != 0 {
		t.Fatal("the approval still waits, or the tool ran")
	}
}

// TestReloadDropsPendingApproval: a conversation closed while an approval
// waits resumes with the turn interrupted and nothing waiting.
func TestReloadDropsPendingApproval(t *testing.T) {
	ran := make(chan struct{}, 10)
	h := newHarness(t, []tool.Tool{changer(ran)}, call("t1", "change"))
	_ = h.conv.Send("go")
	next[*ApprovalRequest](t, h)
	id := h.conv.ID()
	if err := h.conv.Close(); err != nil {
		t.Fatal(err)
	}
	c, loaded, err := Resume(h.config(t), id)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	if loaded.MidTurn || c.Pending() != nil || len(ran) != 0 {
		t.Fatalf("mid-turn %v, pending %+v, ran %d", loaded.MidTurn, c.Pending(), len(ran))
	}
	if last := loaded.Records[len(loaded.Records)-1]; last.Type() != convstore.TypeInterruption {
		t.Fatalf("last record %s", last.Type())
	}
}

// TestModeSurvivesResume: auto mode runs changes without asking, and a
// resume keeps it.
func TestModeSurvivesResume(t *testing.T) {
	ran := make(chan struct{}, 10)
	h := newHarness(t, []tool.Tool{changer(ran)}, call("t1", "change"), text("ok"))
	if err := h.conv.SetMode(ModeAuto); err != nil {
		t.Fatal(err)
	}
	if m := next[*ModeChanged](t, h); m.Mode != ModeAuto || m.Failure != nil {
		t.Fatalf("mode changed %+v", m)
	}
	_ = h.conv.Send("go")
	next[*TurnEnded](t, h)
	if len(ran) != 1 {
		t.Fatal("auto mode did not run the tool")
	}
	id := h.conv.ID()
	_ = h.conv.Close()
	c, _, err := Resume(h.config(t), id)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	if c.Mode() != ModeAuto {
		t.Fatalf("mode after resume %s", c.Mode())
	}
	if err := c.SetMode("yolo"); err == nil {
		t.Fatal("an unknown mode was accepted")
	}
}

// TestOutsideAsksInAuto: a read outside the folder asks even in auto mode,
// and the request names the path.
func TestOutsideAsksInAuto(t *testing.T) {
	h := newHarness(t, nil, pathCall("t1", "../secret"), text("ok"))
	read, _ := tool.Builtin().Get("read")
	if err := h.tools.Register(read); err != nil {
		t.Fatal(err)
	}
	_ = h.conv.SetMode(ModeAuto)
	next[*ModeChanged](t, h)
	_ = h.conv.Send("go")
	req := next[*ApprovalRequest](t, h)
	if len(req.Outside) != 1 || req.Outside[0] != "../secret" {
		t.Fatalf("request %+v", req)
	}
	_ = h.conv.Answer(Answer{ID: "t1", Decision: DecisionNo})
	if r := next[*ToolResult](t, h); r.Content != "denied: the user said no" {
		t.Fatalf("result %+v", r)
	}
}

// TestApprovalShowsDiff: an edit's request carries the old and new text.
func TestApprovalShowsDiff(t *testing.T) {
	h := newHarness(t, nil, providertest.Reply{Events: []provider.Event{
		&provider.ToolUse{ID: "t1", Name: "write", Input: json.RawMessage(`{"path":"a.txt","content":"new"}`)},
		&provider.Stop{Reason: provider.StopToolUse},
	}}, text("ok"))
	write, _ := tool.Builtin().Get("write")
	_ = h.tools.Register(write)
	_ = h.conv.Send("go")
	req := next[*ApprovalRequest](t, h)
	if req.Diff == nil || *req.Diff != (Diff{Path: "a.txt", New: "new"}) {
		t.Fatalf("diff %+v", req.Diff)
	}
	_ = h.conv.Answer(Answer{ID: "t1", Decision: DecisionYes})
	next[*TurnEnded](t, h)
}

// TestFinishedCallKeepsResult: a tool that finishes its work after an
// interrupt, as write and edit do, reports what it did, not "interrupted".
func TestFinishedCallKeepsResult(t *testing.T) {
	started := make(chan struct{})
	stubborn := fnTool{tool.Def{Name: "stubborn"}, func(ctx context.Context) (string, error) {
		close(started)
		<-ctx.Done() // the interrupt lands while the work goes on
		return "wrote it", nil
	}}
	h := newHarness(t, []tool.Tool{stubborn}, call("t1", "stubborn"))
	_ = h.conv.Send("go")
	<-started
	_ = h.conv.Interrupt()
	if r := next[*ToolResult](t, h); r.IsError || r.Content != "wrote it" {
		t.Fatalf("result %+v", r)
	}
	if end := next[*TurnEnded](t, h); end.Reason != EndInterrupted {
		t.Fatalf("turn ended %+v", end)
	}
}

// TestAnswerAfterInterrupt: once a turn is interrupted, its approval no
// longer waits: an answer is refused and Pending is empty.
func TestAnswerAfterInterrupt(t *testing.T) {
	ran := make(chan struct{}, 1)
	h := newHarness(t, []tool.Tool{changer(ran)}, call("t1", "change"))
	_ = h.conv.Send("go")
	next[*ApprovalRequest](t, h)
	_ = h.conv.Interrupt()
	if p := h.conv.Pending(); p != nil {
		t.Fatalf("pending after interrupt: %+v", p)
	}
	if err := h.conv.Answer(Answer{ID: "t1", Decision: DecisionYes}); !errors.Is(err, ErrNoApproval) {
		t.Fatalf("answer after interrupt: %v", err)
	}
	next[*TurnEnded](t, h)
	if len(ran) != 0 {
		t.Fatal("the tool ran")
	}
}
