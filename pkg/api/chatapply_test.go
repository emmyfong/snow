package api

import (
	"reflect"
	"testing"
)

// TestChatApply folds a turn's events into a snapshot.
func TestChatApply(t *testing.T) {
	ev := func(e ChatEvent) *ChatEvent { e.Pane, e.Agent = 1, "a"; return &e }
	diff := &ChatDiff{Path: "a.go", New: "x"}
	tests := []struct {
		name   string
		events []*ChatEvent
		want   ChatSnapshot
	}{
		{
			name: "streamed reply",
			events: []*ChatEvent{
				ev(ChatEvent{Kind: ChatEventTurnStarted, Turn: 1, Text: "hi"}),
				ev(ChatEvent{Kind: ChatEventThinking, Text: "hmm"}),
				ev(ChatEvent{Kind: ChatEventText, Text: "hel"}),
				ev(ChatEvent{Kind: ChatEventText, Text: "lo"}),
				ev(ChatEvent{Kind: ChatEventTurnEnded, Turn: 1, Reason: ChatEndDone}),
			},
			want: ChatSnapshot{Turn: 1, Entries: []ChatEntry{
				{Kind: ChatEntryUser, Turn: 1, Text: "hi"},
				{Kind: ChatEntryThinking, Turn: 1, Text: "hmm"},
				{Kind: ChatEntryAssistant, Turn: 1, Text: "hello"},
			}},
		},
		{
			name: "tool with approval",
			events: []*ChatEvent{
				ev(ChatEvent{Kind: ChatEventTurnStarted, Turn: 1, Text: "go"}),
				ev(ChatEvent{Kind: ChatEventToolCall, Tool: &ChatTool{ID: "t", Name: "write"}}),
				ev(ChatEvent{Kind: ChatEventApproval, Approval: &ChatApproval{ID: "t", Tool: "write", Diff: diff}}),
			},
			want: ChatSnapshot{Running: true, Turn: 1, Entries: []ChatEntry{
				{Kind: ChatEntryUser, Turn: 1, Text: "go"},
				{Kind: ChatEntryTool, Turn: 1, Tool: &ChatTool{ID: "t", Name: "write", Diff: diff}},
			}, Pending: &ChatApproval{ID: "t", Tool: "write", Diff: diff}},
		},
		{
			name: "result clears the approval",
			events: []*ChatEvent{
				ev(ChatEvent{Kind: ChatEventTurnStarted, Turn: 1}),
				ev(ChatEvent{Kind: ChatEventToolCall, Tool: &ChatTool{ID: "t", Name: "run"}}),
				ev(ChatEvent{Kind: ChatEventApproval, Approval: &ChatApproval{ID: "t", Tool: "run"}}),
				ev(ChatEvent{Kind: ChatEventToolResult, Tool: &ChatTool{ID: "t", Result: "no", IsError: true}}),
			},
			want: ChatSnapshot{Running: true, Turn: 1, Entries: []ChatEntry{
				{Kind: ChatEntryTool, Turn: 1, Tool: &ChatTool{ID: "t", Name: "run", Done: true, Result: "no", IsError: true}},
			}},
		},
		{
			name: "interrupt cuts the reply",
			events: []*ChatEvent{
				ev(ChatEvent{Kind: ChatEventTurnStarted, Turn: 2, Text: "x"}),
				ev(ChatEvent{Kind: ChatEventText, Text: "par"}),
				ev(ChatEvent{Kind: ChatEventTurnEnded, Turn: 2, Reason: ChatEndInterrupted}),
			},
			want: ChatSnapshot{Turn: 2, Entries: []ChatEntry{
				{Kind: ChatEntryUser, Turn: 2, Text: "x"},
				{Kind: ChatEntryAssistant, Turn: 2, Text: "par", Cut: true},
				{Kind: ChatEntryInterrupted, Turn: 2},
			}},
		},
		{
			name: "failure",
			events: []*ChatEvent{
				ev(ChatEvent{Kind: ChatEventTurnStarted, Turn: 1, Text: "x"}),
				ev(ChatEvent{Kind: ChatEventTurnEnded, Turn: 1, Reason: ChatEndFailed, Error: &ChatError{Code: ChatErrNetwork}}),
			},
			want: ChatSnapshot{Turn: 1, Entries: []ChatEntry{
				{Kind: ChatEntryUser, Turn: 1, Text: "x"},
				{Kind: ChatEntryError, Turn: 1, Error: &ChatError{Code: ChatErrNetwork}},
			}},
		},
		{
			name: "state, queue, usage, model, mode, title",
			events: []*ChatEvent{
				ev(ChatEvent{Kind: ChatEventState, State: ChatStateTool}),
				ev(ChatEvent{Kind: ChatEventQueue, Queue: []string{"a"}}),
				ev(ChatEvent{Kind: ChatEventUsage, Usage: &ChatUsage{Input: 1, Cost: 0.25, Priced: true}}),
				ev(ChatEvent{Kind: ChatEventUsage, Usage: &ChatUsage{Input: 2, Cost: 0.5, Priced: true}}),
				ev(ChatEvent{Kind: ChatEventModel, Model: "p/m", Effort: "low"}),
				ev(ChatEvent{Kind: ChatEventModel, Model: "p/n", Error: &ChatError{Code: ChatErrSave}}),
				ev(ChatEvent{Kind: ChatEventMode, Mode: "auto"}),
				ev(ChatEvent{Kind: ChatEventTitle, Text: "T"}),
			},
			want: ChatSnapshot{
				State: ChatStateTool, Queue: []string{"a"}, Usage: &ChatUsage{Input: 2, Cost: 0.5, Priced: true}, Cost: 0.75,
				Model: "p/m", Effort: "low", Mode: "auto", Title: "T",
				Entries: []ChatEntry{{Kind: ChatEntryModel, Text: "p/m"}, {Kind: ChatEntryError, Error: &ChatError{Code: ChatErrSave}}},
			},
		},
		{
			name: "a failed rename keeps the title and shows the error",
			events: []*ChatEvent{
				ev(ChatEvent{Kind: ChatEventTitle, Text: "T"}),
				ev(ChatEvent{Kind: ChatEventTitle, Error: &ChatError{Code: ChatErrSave}}),
			},
			want: ChatSnapshot{Title: "T", Entries: []ChatEntry{{Kind: ChatEntryError, Error: &ChatError{Code: ChatErrSave}}}},
		},
		{
			name:   "other pane",
			events: []*ChatEvent{{Pane: 9, Agent: "a", Kind: ChatEventTitle, Text: "T"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := ChatSnapshot{Pane: 1, Agent: "a"}
			for _, e := range tt.events {
				s.Apply(e)
			}
			tt.want.Pane, tt.want.Agent = 1, "a"
			if !reflect.DeepEqual(s, tt.want) {
				t.Fatalf("got  %+v\nwant %+v", s, tt.want)
			}
		})
	}
}

func TestReportedMatches(t *testing.T) {
	tests := []struct {
		requested, reported string
		want                bool
	}{
		{"anthropic/claude-sonnet-5-5", "", true},
		{"anthropic/claude-sonnet-5-5", "claude-sonnet-5-5", true},
		{"anthropic/claude-sonnet-5-5", "claude-sonnet-5-5-20261001", true},
		{"openai/gpt-6.1-sol", "gpt-6.1-sol-2026-09-01", true},
		{"anthropic/claude-sonnet-5-5", "claude-haiku-5-5", false},
		{"anthropic/claude-sonnet-5-5", "claude-sonnet-5-5-mini", false},
		{"anthropic/claude-sonnet-5-5", "claude-sonnet-5", false},
		{"openai/gpt-6.1", "gpt-6.1-sol", false},
	}
	for _, tt := range tests {
		if got := reportedMatches(tt.requested, tt.reported); got != tt.want {
			t.Errorf("reportedMatches(%q, %q) = %v", tt.requested, tt.reported, got)
		}
	}
}

// TestModelMismatchShowsBoth: a reply from another model than requested
// adds one entry that names both, once per turn.
func TestModelMismatchShowsBoth(t *testing.T) {
	s := ChatSnapshot{Pane: 1}
	u := &ChatUsage{Requested: "anthropic/claude-sonnet-5-5", Reported: "claude-haiku-5-5"}
	s.Apply(&ChatEvent{Pane: 1, Kind: ChatEventTurnStarted, Turn: 1, Text: "hi"})
	s.Apply(&ChatEvent{Pane: 1, Kind: ChatEventUsage, Usage: u})
	s.Apply(&ChatEvent{Pane: 1, Kind: ChatEventUsage, Usage: u})
	var got []ChatEntry
	for _, e := range s.Entries {
		if e.Kind == ChatEntryReported {
			got = append(got, e)
		}
	}
	if len(got) != 1 || got[0].Text != "claude-haiku-5-5" || got[0].Requested != "anthropic/claude-sonnet-5-5" {
		t.Fatalf("entries %+v", s.Entries)
	}
	s.Apply(&ChatEvent{Pane: 1, Kind: ChatEventUsage, Usage: &ChatUsage{Requested: u.Requested, Reported: "claude-sonnet-5-5-20261001"}})
	if n := len(s.Entries); s.Entries[n-1].Kind != ChatEntryReported || n != 2 {
		t.Fatalf("a dated ID added an entry: %+v", s.Entries)
	}
}
