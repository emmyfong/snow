package usage

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/pkg/api"
)

var at = time.Date(2026, 11, 17, 14, 2, 0, 0, time.UTC)

// runtimeConversation is a conversation from an external runtime.
func runtimeConversation(t *testing.T, records ...convstore.Record) *convstore.Conversation {
	t.Helper()
	return &convstore.Conversation{
		Meta: convstore.Meta{ID: "conv-1", Folder: "/work", Model: "claude-sonnet-4-5",
			Runtime: &convstore.RuntimeMeta{Kind: "claude-code", Connection: "cc"}},
		Records: records,
	}
}

// directConversation is a conversation from Snow's direct-API loop.
func directConversation(records ...convstore.Record) *convstore.Conversation {
	return &convstore.Conversation{
		Meta:    convstore.Meta{ID: "conv-2", Folder: "/work", Model: "anthropic/claude-sonnet-4-5"},
		Records: records,
	}
}

// event returns the record a runtime display event becomes.
func event(t *testing.T, e api.ChatEvent) convstore.Record {
	t.Helper()
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	return &convstore.RuntimeEvent{At: at, Display: b}
}

// TestUsageCumulativeEvents: a runtime's cost is a session estimate, so a
// later result replaces the total instead of adding to it. Tokens stay per
// turn.
func TestUsageCumulativeEvents(t *testing.T) {
	conv := runtimeConversation(t,
		event(t, api.ChatEvent{Kind: api.ChatEventTurnStarted, Turn: 1, Text: "one"}),
		event(t, api.ChatEvent{Kind: api.ChatEventUsage, Usage: &api.ChatUsage{
			Requested: "claude-sonnet-4-5", Input: 100, Output: 10, CacheRead: 1000, CacheWrite: 50,
			Cost: 0.10, Priced: true, Cumulative: true, Scope: "main-agent"}}),
		event(t, api.ChatEvent{Kind: api.ChatEventTurnEnded, Turn: 1, Reason: api.ChatEndDone}),
		event(t, api.ChatEvent{Kind: api.ChatEventTurnStarted, Turn: 2, Text: "two"}),
		event(t, api.ChatEvent{Kind: api.ChatEventUsage, Usage: &api.ChatUsage{
			Requested: "claude-sonnet-4-5", Input: 200, Output: 20, CacheRead: 2000, CacheWrite: 60,
			Cost: 0.25, Priced: true, Cumulative: true, Scope: "main-agent"}}),
		event(t, api.ChatEvent{Kind: api.ChatEventTurnEnded, Turn: 2, Reason: api.ChatEndDone}),
	)
	st := Account(conv)
	if st.Turns != 2 || len(st.Rows) != 2 {
		t.Fatalf("turns %d, rows %d; want 2 and 2", st.Turns, len(st.Rows))
	}
	if got := st.Totals.Cost; math.Abs(got-0.25) > 1e-9 {
		t.Fatalf("cost %v, want the session estimate 0.25, not the sum 0.35", got)
	}
	if !st.Totals.Priced || !st.Totals.Cumulative {
		t.Fatalf("totals %+v; want a priced cumulative estimate", st.Totals)
	}
	if st.Totals.Input != 300 || st.Totals.Output != 30 || st.Totals.CacheRead != 3000 || st.Totals.CacheWrite != 110 {
		t.Fatalf("totals %+v; want the two turns added", st.Totals)
	}
	// A runtime row has no cost of its own: the estimate is session-wide.
	if st.Rows[0].Turn != 2 || st.Rows[1].Turn != 1 {
		t.Fatalf("rows %d, %d; want newest first 2, 1", st.Rows[0].Turn, st.Rows[1].Turn)
	}
	for _, r := range st.Rows {
		if r.Priced || r.Cost != 0 {
			t.Fatalf("row %+v; want no per-turn cost for a cumulative source", r)
		}
		if r.Scope != "main-agent" {
			t.Fatalf("row scope %q, want main-agent", r.Scope)
		}
	}
	// A session estimate cannot be split between models, so no model row
	// may claim a cost.
	for _, m := range st.Models {
		if m.Priced || m.Cost != 0 {
			t.Fatalf("model %+v; want no per-model cost for a cumulative source", m)
		}
	}
}

// TestUsageDuplicateEvents: a runtime that repeats the same result must not
// double count, even when the repeat arrives after the turn ended.
func TestUsageDuplicateEvents(t *testing.T) {
	usage := &api.ChatUsage{Requested: "claude-sonnet-4-5", Input: 200, Output: 20,
		Cost: 0.25, Priced: true, Cumulative: true}
	conv := runtimeConversation(t,
		event(t, api.ChatEvent{Kind: api.ChatEventTurnStarted, Turn: 1, Text: "one"}),
		event(t, api.ChatEvent{Kind: api.ChatEventUsage, Usage: usage}),
		event(t, api.ChatEvent{Kind: api.ChatEventTurnEnded, Turn: 1, Reason: api.ChatEndDone}),
		event(t, api.ChatEvent{Kind: api.ChatEventUsage, Usage: usage}),
	)
	st := Account(conv)
	if st.Turns != 1 || len(st.Rows) != 1 {
		t.Fatalf("turns %d, rows %d; want the repeated result to stay one turn", st.Turns, len(st.Rows))
	}
	if st.Totals.Input != 200 || st.Totals.Output != 20 {
		t.Fatalf("totals %+v; want the duplicate tokens counted once", st.Totals)
	}
	if got := st.Totals.Cost; math.Abs(got-0.25) > 1e-9 {
		t.Fatalf("cost %v, want 0.25", got)
	}

	// The same result may arrive after a new turn started. It belongs to
	// the turn that already counted it, so the new turn stays unknown.
	interleaved := runtimeConversation(t,
		event(t, api.ChatEvent{Kind: api.ChatEventTurnStarted, Turn: 1, Text: "one"}),
		event(t, api.ChatEvent{Kind: api.ChatEventUsage, Usage: usage}),
		event(t, api.ChatEvent{Kind: api.ChatEventTurnEnded, Turn: 1, Reason: api.ChatEndDone}),
		event(t, api.ChatEvent{Kind: api.ChatEventTurnStarted, Turn: 2, Text: "two"}),
		event(t, api.ChatEvent{Kind: api.ChatEventUsage, Usage: usage}),
		event(t, api.ChatEvent{Kind: api.ChatEventTurnEnded, Turn: 2, Reason: api.ChatEndDone}),
	)
	ist := Account(interleaved)
	if ist.Turns != 2 || ist.Unknown != 1 || len(ist.Rows) != 2 {
		t.Fatalf("interleaved turns %d, unknown %d, rows %d; want 2, 1, 2", ist.Turns, ist.Unknown, len(ist.Rows))
	}
	if ist.Totals.Input != 200 || ist.Totals.Output != 20 {
		t.Fatalf("interleaved totals %+v; want the repeat counted once", ist.Totals)
	}
	if !ist.Rows[0].Unknown || ist.Rows[0].Turn != 2 {
		t.Fatalf("newest row %+v; want the second turn unknown", ist.Rows[0])
	}
}

// TestUsageCacheCategories: direct-API and runtime usage keep input, output,
// and cache read and write apart, with input never counting cached tokens.
func TestUsageCacheCategories(t *testing.T) {
	direct := directConversation(
		&convstore.User{At: at, Text: "read it"},
		&convstore.Assistant{At: at, Model: "claude-sonnet-4-5", Stop: convstore.StopEndTurn,
			Usage: &convstore.Usage{Input: 1000, Output: 500, CacheRead: 10000, CacheWrite: 2000}},
	)
	runtime := runtimeConversation(t,
		event(t, api.ChatEvent{Kind: api.ChatEventTurnStarted, Turn: 1, Text: "read it"}),
		event(t, api.ChatEvent{Kind: api.ChatEventUsage, Usage: &api.ChatUsage{
			Requested: "claude-sonnet-4-5", Input: 1000, Output: 500, CacheRead: 10000, CacheWrite: 2000,
			Cost: 0.50, Priced: true, Cumulative: true}}),
		event(t, api.ChatEvent{Kind: api.ChatEventTurnEnded, Turn: 1, Reason: api.ChatEndDone}),
	)

	for name, st := range map[string]api.UsageState{"direct": Account(direct), "runtime": Account(runtime)} {
		want := api.UsageTotals{Input: 1000, Output: 500, CacheRead: 10000, CacheWrite: 2000}
		if st.Totals.Input != want.Input || st.Totals.Output != want.Output ||
			st.Totals.CacheRead != want.CacheRead || st.Totals.CacheWrite != want.CacheWrite {
			t.Fatalf("%s totals %+v; want %+v", name, st.Totals, want)
		}
		if st.Totals.Input == 1000+10000+2000 {
			t.Fatalf("%s input includes cached tokens", name)
		}
	}
	// The direct path has list prices, and adds up its replies.
	directState := Account(direct)
	if !directState.Totals.Priced {
		t.Fatalf("direct totals %+v; want a priced estimate", directState.Totals)
	}
	if got := directState.Totals.Cost; math.Abs(got-0.021) > 1e-9 {
		t.Fatalf("direct cost %v, want 0.021", got)
	}
}

// TestUsageInterruptedUnknown: a turn that lost its usage is unknown, not
// zero, and the totals are a lower bound.
func TestUsageInterruptedUnknown(t *testing.T) {
	used := &convstore.Usage{Input: 1000, Output: 200}
	conv := directConversation(
		&convstore.User{At: at, Text: "one"},
		&convstore.Assistant{At: at, Model: "claude-sonnet-4-5", Stop: convstore.StopEndTurn, Usage: used},
		&convstore.User{At: at.Add(time.Minute), Text: "two"},
		&convstore.Interruption{At: at.Add(time.Minute), Reason: "stopped"},
	)
	st := Account(conv)
	if st.Turns != 2 || st.Unknown != 1 || len(st.Rows) != 2 {
		t.Fatalf("turns %d, unknown %d, rows %d; want 2, 1, 2", st.Turns, st.Unknown, len(st.Rows))
	}
	if !st.Rows[0].Unknown || !st.Rows[0].Interrupted {
		t.Fatalf("newest row %+v; want the stopped turn unknown", st.Rows[0])
	}
	if st.Rows[1].Unknown {
		t.Fatalf("first row %+v; want its usage known", st.Rows[1])
	}
	if !st.Totals.LowerBound {
		t.Fatalf("totals %+v; want a lower bound", st.Totals)
	}
	if st.Totals.Input != 1000 || st.Totals.Output != 200 {
		t.Fatalf("totals %+v; the unknown turn must not be zeroed into the known one", st.Totals)
	}
	found := false
	for _, n := range st.Notes {
		if n == "1 turn has no reported usage; totals are a lower bound." {
			found = true
		}
	}
	if !found {
		t.Fatalf("notes %q; want the unknown-turn note", st.Notes)
	}

	// A runtime turn with no result at all is unknown too.
	rt := runtimeConversation(t,
		event(t, api.ChatEvent{Kind: api.ChatEventTurnStarted, Turn: 1, Text: "one"}),
	)
	rst := Account(rt)
	if rst.Unknown != 1 || !rst.Rows[0].Unknown || !rst.Rows[0].Interrupted {
		t.Fatalf("runtime state %+v; want one unknown interrupted turn", rst)
	}
	if rst.Totals.Priced {
		t.Fatalf("runtime totals %+v; want no cost without a result", rst.Totals)
	}
}
