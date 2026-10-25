package server

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/emmyfong/snow/internal/agent"
	"github.com/emmyfong/snow/pkg/api"
)

// TestChatEventMapping: each agent event becomes its protocol event, with
// the pane and the agent ID.
func TestChatEventMapping(t *testing.T) {
	fail := &agent.Failure{Kind: agent.FailRateLimited, Message: "slow", RetryAfter: 42 * time.Second, Choices: []agent.Choice{agent.ChoiceWait, agent.ChoiceRetry}}
	tests := []struct {
		body agent.Body
		kind string
		ok   func(*api.ChatEvent) bool
	}{
		{&agent.TurnStarted{Turn: 2, Text: "hi"}, api.ChatEventTurnStarted, func(e *api.ChatEvent) bool { return e.Turn == 2 && e.Text == "hi" }},
		{&agent.TurnEnded{Turn: 2, Reason: agent.EndFailed, Failure: fail}, api.ChatEventTurnEnded, func(e *api.ChatEvent) bool {
			return e.Reason == api.ChatEndFailed && e.Error.Code == api.ChatErrRateLimited && e.Error.RetryAfter == 42 &&
				len(e.Error.Choices) == 2 && e.Error.Choices[0] == api.ChatChoiceWait
		}},
		{&agent.TextDelta{Text: "x"}, api.ChatEventText, func(e *api.ChatEvent) bool { return e.Text == "x" }},
		{&agent.ThinkingDelta{Text: "y"}, api.ChatEventThinking, func(e *api.ChatEvent) bool { return e.Text == "y" }},
		{&agent.ToolCall{ID: "t", Name: "read", Input: json.RawMessage(`{"path":"a"}`)}, api.ChatEventToolCall, func(e *api.ChatEvent) bool { return e.Tool.Name == "read" && string(e.Tool.Input) == `{"path":"a"}` }},
		{&agent.ToolResult{ID: "t", Content: "ok", IsError: true}, api.ChatEventToolResult, func(e *api.ChatEvent) bool { return e.Tool.Done && e.Tool.IsError && e.Tool.Result == "ok" }},
		{&agent.ApprovalRequest{ID: "t", Tool: "write", Diff: &agent.Diff{Path: "a", New: "b"}, Outside: []string{"/x"}}, api.ChatEventApproval, func(e *api.ChatEvent) bool {
			return e.Approval.Diff.New == "b" && e.Approval.Outside[0] == "/x"
		}},
		{&agent.StateChanged{State: agent.StateTool}, api.ChatEventState, func(e *api.ChatEvent) bool { return e.State == api.ChatStateTool }},
		{&agent.QueueChanged{Messages: []string{"a"}}, api.ChatEventQueue, func(e *api.ChatEvent) bool { return len(e.Queue) == 1 }},
		{&agent.Usage{Requested: "anthropic/claude-opus-5-5", Input: 3, Priced: true, Cost: 0.5}, api.ChatEventUsage, func(e *api.ChatEvent) bool {
			return e.Usage.Window == 1_000_000 && e.Usage.Cost == 0.5
		}},
		{&agent.ModelChanged{Model: "p/m", Effort: "low"}, api.ChatEventModel, func(e *api.ChatEvent) bool { return e.Model == "p/m" && e.Error == nil }},
		{&agent.ModeChanged{Mode: agent.ModeAuto, Failure: &agent.Failure{Kind: agent.FailSave}}, api.ChatEventMode, func(e *api.ChatEvent) bool { return e.Error.Code == api.ChatErrSave }},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			e := chatEvent(7, agent.Event{Agent: "c1", Body: tt.body})
			if e == nil || e.Pane != 7 || e.Agent != "c1" || e.Kind != tt.kind || !tt.ok(e) {
				t.Fatalf("got %+v", e)
			}
		})
	}
	if e := chatEvent(7, agent.Event{Agent: "c1", Body: &agent.Question{ID: "q"}}); e != nil {
		t.Fatalf("question mapped to %+v; the protocol has no kind for it yet", e)
	}
}

// TestCapsKeepFrames: long results, diffs, and inputs are cut on a
// character boundary, and an input stays valid JSON.
func TestCapsKeepFrames(t *testing.T) {
	long := strings.Repeat("é", api.MaxToolResult)
	if got := capText(long, api.MaxToolResult); len(got) > api.MaxToolResult || !utf8.ValidString(got) || !strings.HasSuffix(got, "(cut)") {
		t.Fatalf("capText: %d bytes, valid %v", len(got), utf8.ValidString(got))
	}
	if got := capText("short", 100); got != "short" {
		t.Fatalf("capText short: %q", got)
	}
	in, _ := json.Marshal(map[string]string{"path": "a.go", "content": strings.Repeat("x", 1<<20)})
	got := capInput(in)
	var m map[string]string
	if err := json.Unmarshal(got, &m); err != nil || m["path"] != "a.go" || len(m["content"]) > maxInputValue {
		t.Fatalf("capInput: %d bytes, %v", len(got), err)
	}
	if got := capInput(json.RawMessage(`"` + strings.Repeat("x", 1<<16) + `"`)); got != nil {
		t.Fatalf("a long non-object input should be dropped, got %d bytes", len(got))
	}
}

// TestFitSnapshot: a snapshot too large for a frame leaves out its oldest
// entries and counts them.
func TestFitSnapshot(t *testing.T) {
	snap := api.ChatSnapshot{Pane: 1, Agent: "a", Earlier: 2}
	for range 200 {
		snap.Entries = append(snap.Entries, api.ChatEntry{Kind: api.ChatEntryAssistant, Text: strings.Repeat("x", 10<<10)})
	}
	got := fitSnapshot(snap)
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) > snapshotBudget || got.Earlier+len(got.Entries) != 202 || len(got.Entries) == 0 {
		t.Fatalf("fit: %d bytes, earlier %d, %d entries", len(b), got.Earlier, len(got.Entries))
	}
	if small := fitSnapshot(api.ChatSnapshot{Pane: 1, Entries: snap.Entries[:3]}); small.Earlier != 0 || len(small.Entries) != 3 {
		t.Fatalf("a small snapshot changed: %+v", small)
	}
}

// TestApprovalEventFitsFrame: text that escapes to six bytes per byte in
// JSON passes the per-field caps, so the event itself is measured and cut
// further, with the cut marked.
func TestApprovalEventFitsFrame(t *testing.T) {
	bad := strings.Repeat("\x01", maxDiffText)
	in, _ := json.Marshal(map[string]string{"path": "a.go", "content": strings.Repeat("\x01", maxInputValue)})
	for _, body := range []agent.Body{
		&agent.ApprovalRequest{ID: "w1", Tool: "write", Input: in, Diff: &agent.Diff{Path: "a.go", Old: bad, New: bad}},
		&agent.ToolCall{ID: "w1", Name: "write", Input: in},
	} {
		e := chatEvent(1, agent.Event{Agent: "c1", Body: body})
		b, err := json.Marshal(e)
		if err != nil || len(b) > eventBudget {
			t.Fatalf("%T: %d bytes, budget %d, %v", body, len(b), eventBudget, err)
		}
		if a := e.Approval; a != nil && (a.Diff == nil || !strings.HasSuffix(a.Diff.New, "(cut)")) {
			t.Fatalf("the cut diff is not marked: %+v", a.Diff)
		}
	}
}
