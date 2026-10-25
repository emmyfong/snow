package agent

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providertest"
	"github.com/emmyfong/snow/internal/tool"
)

// TestTurnToolLoop: the model calls a tool twice, then ends. Every record
// is saved in order, and each request carries the whole exchange so far.
func TestTurnToolLoop(t *testing.T) {
	h := newHarness(t, []tool.Tool{echo}, call("t1", "echo"), call("t2", "echo"), text("Done."))
	if err := h.conv.Send("Check twice"); err != nil {
		t.Fatal(err)
	}
	if end := next[*TurnEnded](t, h); end.Reason != EndDone {
		t.Fatalf("turn ended %+v", end)
	}
	want := []convstore.Type{
		convstore.TypeUser,
		convstore.TypeAssistant, convstore.TypeToolResult,
		convstore.TypeAssistant, convstore.TypeToolResult,
		convstore.TypeAssistant,
	}
	recs := h.records(t)
	if got := types(recs); !reflect.DeepEqual(got, want) {
		t.Fatalf("records %v, want %v", got, want)
	}
	if c, _ := h.store.Load(h.conv.ID()); c.Meta.Created.IsZero() {
		t.Fatal("the meta record has no creation time")
	}
	if r := recs[2].(*convstore.ToolResult); r.ToolUseID != "t1" || r.Content != "echoed" {
		t.Fatalf("first result %+v", r)
	}
	if a := recs[5].(*convstore.Assistant); a.Model != "model-1-20261001" || a.Stop != convstore.StopEndTurn || a.Usage == nil {
		t.Fatalf("final reply %+v", a)
	}
	reqs := h.fake.Requests()
	if len(reqs) != 3 || len(reqs[2].Messages) != 5 || reqs[2].Model != "model-1" || len(reqs[2].Tools) != 1 {
		t.Fatalf("requests %d, last has %d messages", len(reqs), len(reqs[len(reqs)-1].Messages))
	}
	if b, ok := reqs[2].Messages[4].Content[0].(*provider.ToolResult); !ok || b.ToolUseID != "t2" {
		t.Fatalf("last message %#v", reqs[2].Messages[4])
	}
}

// TestEventsCarryAgentID: every kind of event names the main agent and
// the conversation.
func TestEventsCarryAgentID(t *testing.T) {
	h := newHarness(t, nil)
	bodies := []Body{
		&TurnStarted{}, &TurnEnded{}, &TextDelta{}, &ThinkingDelta{}, &ToolCall{}, &ToolResult{},
		&ApprovalRequest{}, &Question{}, &TasksChanged{}, &Usage{}, &StateChanged{}, &QueueChanged{},
	}
	for _, b := range bodies {
		h.conv.emit(b)
		e := <-h.events
		if e.Body != b || e.Agent != ID(h.conv.ID()) || e.Conversation != h.conv.ID() || e.At.IsZero() {
			t.Errorf("%T: event %+v", b, e)
		}
	}
}

// TestQueuedMessageSentAfterTurn: a message sent during a turn starts the
// next turn when the first ends.
func TestQueuedMessageSentAfterTurn(t *testing.T) {
	release, ctxErr := make(chan struct{}), make(chan error, 1)
	h := newHarness(t, []tool.Tool{gate(release, ctxErr)}, call("t1", "gate"), text("First."), text("Second."))
	if err := h.conv.Send("one"); err != nil {
		t.Fatal(err)
	}
	next[*ToolCall](t, h)
	if err := h.conv.Send("two"); err != nil {
		t.Fatal(err)
	}
	if q := next[*QueueChanged](t, h); !reflect.DeepEqual(q.Messages, []string{"two"}) {
		t.Fatalf("queue %v", q.Messages)
	}
	close(release)
	if end := next[*TurnEnded](t, h); end.Turn != 1 || end.Reason != EndDone {
		t.Fatalf("first turn ended %+v", end)
	}
	if end := next[*TurnEnded](t, h); end.Turn != 2 || end.Reason != EndDone {
		t.Fatalf("second turn ended %+v", end)
	}
	reqs := h.fake.Requests()
	last := reqs[len(reqs)-1].Messages
	if len(reqs) != 3 || last[len(last)-1].Content[0].(*provider.Text).Text != "two" {
		t.Fatalf("%d requests; the last ends with %#v", len(reqs), last[len(last)-1])
	}
}

// TestInterruptSavesPartial: an interrupt while text streams saves the
// text so far, which loads as a cut-off reply, and marks the turn
// interrupted.
func TestInterruptSavesPartial(t *testing.T) {
	waiting := providertest.Reply{Events: []provider.Event{&provider.TextDelta{Text: "Hel"}}, Wait: true}
	h := newHarness(t, nil, waiting)
	if err := h.conv.Send("hi"); err != nil {
		t.Fatal(err)
	}
	next[*TextDelta](t, h)
	if err := h.conv.Interrupt(); err != nil {
		t.Fatal(err)
	}
	if end := next[*TurnEnded](t, h); end.Reason != EndInterrupted {
		t.Fatalf("turn ended %+v", end)
	}
	recs := h.records(t)
	want := []convstore.Type{convstore.TypeUser, convstore.TypeAssistant, convstore.TypeInterruption}
	if got := types(recs); !reflect.DeepEqual(got, want) {
		t.Fatalf("records %v, want %v", got, want)
	}
	if a := recs[1].(*convstore.Assistant); !a.CutOff || a.Blocks[0].Text != "Hel" {
		t.Fatalf("partial reply %+v", a)
	}
}

// TestInterruptCancelsTool: an interrupt cancels a running tool's context,
// and the call gets an "interrupted" result so the exchange stays valid.
func TestInterruptCancelsTool(t *testing.T) {
	release, ctxErr := make(chan struct{}), make(chan error, 1)
	h := newHarness(t, []tool.Tool{gate(release, ctxErr)}, call("t1", "gate"))
	if err := h.conv.Send("go"); err != nil {
		t.Fatal(err)
	}
	if s := next[*StateChanged](t, h); s.State != StateStreaming {
		t.Fatalf("state %v", s.State)
	}
	if s := next[*StateChanged](t, h); s.State != StateTool {
		t.Fatalf("state %v", s.State)
	}
	if err := h.conv.Interrupt(); err != nil {
		t.Fatal(err)
	}
	if err := <-ctxErr; !errors.Is(err, context.Canceled) {
		t.Fatalf("tool context: %v", err)
	}
	if end := next[*TurnEnded](t, h); end.Reason != EndInterrupted {
		t.Fatalf("turn ended %+v", end)
	}
	recs := h.records(t)
	if r, ok := recs[2].(*convstore.ToolResult); !ok || !r.IsError || r.Content != "interrupted by the user" {
		t.Fatalf("result %#v", recs[2])
	}
	if recs[len(recs)-1].Type() != convstore.TypeInterruption {
		t.Fatalf("records %v", types(recs))
	}
}

// TestSaveFailureStopsBeforeSend: a user message that cannot be saved is
// never sent, and the failure names the file.
func TestSaveFailureStopsBeforeSend(t *testing.T) {
	h := newHarness(t, nil, text("never"))
	path := h.conv.w.Path()
	if err := h.conv.w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := h.conv.Send("hi"); err != nil {
		t.Fatal(err)
	}
	end := next[*TurnEnded](t, h)
	if end.Reason != EndFailed || end.Failure == nil || end.Failure.Kind != FailSave || !strings.Contains(end.Failure.Message, path) {
		t.Fatalf("turn ended %+v (%+v)", end, end.Failure)
	}
	if n := len(h.fake.Requests()); n != 0 {
		t.Fatalf("the provider got %d requests", n)
	}
}
