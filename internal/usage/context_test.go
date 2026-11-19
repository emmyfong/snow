package usage

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/pkg/api"
)

// toolCall returns the assistant record of one tool call.
func toolCall(t *testing.T, id, name string, input map[string]any) *convstore.Assistant {
	t.Helper()
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	return &convstore.Assistant{At: at, Stop: convstore.StopToolUse,
		Blocks: []convstore.Block{{Kind: convstore.BlockToolUse, ID: id, Name: name, Input: raw}}}
}

// TestContextUnavailableDoesNotEstimateHiddenPrompt: a runtime's occupancy
// is unavailable, and its token counters never become an estimated prompt.
func TestContextUnavailableDoesNotEstimateHiddenPrompt(t *testing.T) {
	runtime := runtimeConversation(t,
		event(t, api.ChatEvent{Kind: api.ChatEventTurnStarted, Turn: 1, Text: "one"}),
		event(t, api.ChatEvent{Kind: api.ChatEventUsage, Usage: &api.ChatUsage{
			Requested: "claude-sonnet-4-5", Input: 200, Output: 20, CacheRead: 5000, CacheWrite: 100,
			Cost: 0.25, Priced: true, Cumulative: true, Window: 200_000}}),
		event(t, api.ChatEvent{Kind: api.ChatEventTurnEnded, Turn: 1, Reason: api.ChatEndDone}),
	)
	st := Context(runtime)
	if st.Occupancy.Known {
		t.Fatalf("occupancy %+v; a runtime's occupancy is unavailable", st.Occupancy)
	}
	if st.Occupancy.Tokens != 0 || st.Occupancy.Window != 0 {
		t.Fatalf("occupancy %+v; the runtime's usage must not become a hidden-prompt estimate", st.Occupancy)
	}
	if st.Occupancy.Reason == "" {
		t.Fatal("occupancy has no reason")
	}
	if !strings.Contains(st.Occupancy.Reason, "does not estimate") {
		t.Fatalf("reason %q; want it to say Snow does not estimate the hidden prompt", st.Occupancy.Reason)
	}
	if err := st.Validate(); err != nil {
		t.Fatalf("state does not validate: %v", err)
	}

	// The direct path reports the last prompt's tokens over a known window.
	direct := directConversation(
		&convstore.User{At: at, Text: "read it"},
		&convstore.Assistant{At: at, Model: "claude-sonnet-4-5", Stop: convstore.StopEndTurn,
			Usage: &convstore.Usage{Input: 1000, Output: 50, CacheRead: 2000, CacheWrite: 100}},
	)
	dc := Context(direct)
	if !dc.Occupancy.Known || dc.Occupancy.Tokens != 3100 || dc.Occupancy.Window != 200_000 {
		t.Fatalf("direct occupancy %+v; want 3100 of 200000", dc.Occupancy)
	}
	if err := dc.Validate(); err != nil {
		t.Fatalf("direct state does not validate: %v", err)
	}

	// A measured prompt larger than the table's window still draws. The
	// count is a fact; the window may be out of date.
	large := directConversation(
		&convstore.User{At: at, Text: "read a big file"},
		&convstore.Assistant{At: at, Model: "claude-sonnet-4-5", Stop: convstore.StopEndTurn,
			Usage: &convstore.Usage{Input: 500_000}},
	)
	lc := Context(large)
	if !lc.Occupancy.Known || lc.Occupancy.Tokens != 500_000 || lc.Occupancy.Window != 200_000 {
		t.Fatalf("large occupancy %+v; want the measured 500000 of 200000", lc.Occupancy)
	}
	if err := lc.Validate(); err != nil {
		t.Fatalf("large state does not validate: %v", err)
	}
	found := false
	for _, n := range lc.Notes {
		if strings.Contains(n, "out of date") {
			found = true
		}
	}
	if !found {
		t.Fatalf("notes %q; want the stale-window note", lc.Notes)
	}
}

// TestRepeatedReadFreshness: two identical reads are unchanged; a read
// after an edit differs and is reported as changed.
func TestRepeatedReadFreshness(t *testing.T) {
	conv := directConversation(
		&convstore.User{At: at, Text: "one"},
		toolCall(t, "a1", "read", map[string]any{"path": "a.go", "offset": 1, "limit": 20}),
		&convstore.ToolResult{At: at, ToolUseID: "a1", Content: "1: same"},
		&convstore.Assistant{At: at, Stop: convstore.StopEndTurn},

		&convstore.User{At: at, Text: "two"},
		toolCall(t, "a2", "read", map[string]any{"path": "a.go", "offset": 1, "limit": 20}),
		&convstore.ToolResult{At: at, ToolUseID: "a2", Content: "1: same"},
		toolCall(t, "b1", "read", map[string]any{"path": "b.go"}),
		&convstore.ToolResult{At: at, ToolUseID: "b1", Content: "1: before"},
		toolCall(t, "e1", "edit", map[string]any{"path": "b.go", "old": "before", "new": "after"}),
		&convstore.ToolResult{At: at, ToolUseID: "e1", Content: "edited b.go"},
		toolCall(t, "b2", "read", map[string]any{"path": "b.go"}),
		&convstore.ToolResult{At: at, ToolUseID: "b2", Content: "1: after"},
		&convstore.Assistant{At: at, Stop: convstore.StopEndTurn},
	)
	st := Context(conv)
	if err := st.Validate(); err != nil {
		t.Fatalf("state does not validate: %v", err)
	}

	reads := map[string]api.ContextRead{}
	for _, r := range st.Reads {
		reads[r.Path] = r
	}
	a, ok := reads["a.go"]
	if !ok {
		t.Fatalf("reads %+v; want a.go read twice", st.Reads)
	}
	if a.Reads != 2 || a.Same != 1 || a.Changed != 0 || a.Result != "unchanged" {
		t.Fatalf("a.go %+v; want two unchanged reads", a)
	}
	if a.Range != "1-20" {
		t.Fatalf("a.go range %q, want 1-20", a.Range)
	}
	b, ok := reads["b.go"]
	if !ok {
		t.Fatalf("reads %+v; want b.go read twice", st.Reads)
	}
	if b.Reads != 2 || b.Same != 0 || b.Changed != 1 || b.Result != "changed" {
		t.Fatalf("b.go %+v; want a changed read after the edit", b)
	}
	if b.Evidence != "tool result text" {
		t.Fatalf("b.go evidence %q, want the result text", b.Evidence)
	}

	sources := map[string]api.ContextSource{}
	for _, s := range st.Sources {
		sources[s.Path] = s
	}
	if s := sources["b.go"]; !s.Changed || s.Reads != 2 {
		t.Fatalf("source b.go %+v; want 2 reads and a change", s)
	}
	if s := sources["a.go"]; s.Changed || s.Reads != 2 {
		t.Fatalf("source a.go %+v; want 2 reads and no change", s)
	}

	// A failed read is not evidence of a context source.
	failed := directConversation(
		&convstore.User{At: at, Text: "one"},
		toolCall(t, "a1", "read", map[string]any{"path": "gone.go"}),
		&convstore.ToolResult{At: at, ToolUseID: "a1", Content: "no such file", IsError: true},
	)
	if fs := Context(failed); len(fs.Sources) != 0 || len(fs.Reads) != 0 {
		t.Fatalf("failed read became a source: %+v", fs)
	}
}
