// Package usage accounts for the usage a conversation's records prove. It
// reads the same records the chat pane replays, so a usage pane rebuilds
// after a restore or a restart without new persistence.
package usage

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/pkg/api"
)

// Account returns the usage pane state of loaded's conversation.
func Account(loaded *convstore.Conversation) api.UsageState {
	p := parse(loaded)
	st := api.UsageState{
		Folder:       loaded.Meta.Folder,
		Conversation: loaded.Meta.ID,
		Title:        title(loaded),
		Model:        cut(p.model),
		Turns:        len(p.turns),
	}
	if loaded.Meta.Runtime != nil {
		st.Runtime = loaded.Meta.Runtime.Kind
	}

	var (
		models     = map[string]*modelTotal{}
		session    float64 // the runtime's session estimate, replacing
		sessionSet bool
		addCost    float64 // the sum of priced replies that are not cumulative
		priced     bool
		partial    bool // some usage had no price, so a total is a lower bound
		unpriced   bool // usage arrived, but Snow has no price for the model
	)
	rows := make([]api.UsageTurn, 0, min(len(p.turns), api.MaxUsageTurns))
	for _, t := range p.turns {
		row := rowOf(p, t)
		if row.Unknown {
			st.Unknown++
		}
		if row.Incomplete {
			st.Incomplete++
		}
		st.Totals.Input += row.Input
		st.Totals.Output += row.Output
		st.Totals.CacheRead += row.CacheRead
		st.Totals.CacheWrite += row.CacheWrite

		if p.runtime {
			if u := t.usage; u != nil {
				addModel(models, modelOfTurn(p, t), row, u.Priced)
				switch {
				case u.Priced && u.Cumulative:
					session, sessionSet, priced = u.Cost, true, true
				case u.Priced:
					addCost, priced = addCost+u.Cost, true
				default:
					unpriced = unpriced || hasTokens(u)
				}
			}
		} else {
			for _, r := range t.replies {
				if r.usage == nil {
					continue
				}
				m := addModel(models, modelOfReply(r), usageRow(r.usage), r.usage.Priced)
				if r.usage.Priced {
					addCost, priced = addCost+r.usage.Cost, true
					m.model.Cost += r.usage.Cost
				} else {
					unpriced = unpriced || hasTokens(r.usage)
				}
			}
		}

		rows = append(rows, row)
	}
	partial = unpriced && priced
	if len(rows) > api.MaxUsageTurns {
		// A long task's recent turns matter most.
		st.Earlier = len(rows) - api.MaxUsageTurns
		rows = rows[st.Earlier:]
	}
	slices.Reverse(rows)
	st.Rows = rows
	st.Totals.Cost = addCost
	if sessionSet {
		st.Totals.Cost += session
		st.Totals.Cumulative = true
	}
	st.Totals.Priced = priced
	st.Totals.LowerBound = st.Unknown > 0 || st.Incomplete > 0 || partial

	st.Models = sortedModels(models)
	if len(st.Models) > api.MaxUsageModels {
		st.Models = st.Models[:api.MaxUsageModels]
	}
	st.Notes = notes(p, st, partial)
	return st
}

// rowOf returns one turn's row.
func rowOf(p *parsed, t *turn) api.UsageTurn {
	row := api.UsageTurn{Turn: t.number, At: t.at, Interrupted: t.interrupted}
	if p.runtime {
		row.Model = cut(modelOfTurn(p, t))
		if t.usage == nil {
			row.Unknown = true
			return row
		}
		copyUsage(&row, t.usage)
		// A runtime reports one session estimate, so a turn has no cost of
		// its own unless the event was additive.
		if !t.usage.Cumulative {
			row.Cost, row.Priced = t.usage.Cost, t.usage.Priced
		}
		return row
	}

	known, priced := 0, true
	for _, r := range t.replies {
		row.Model = cut(modelOfReply(r))
		if r.usage == nil {
			continue
		}
		known++
		row.Input += r.usage.Input
		row.Output += r.usage.Output
		row.CacheRead += r.usage.CacheRead
		row.CacheWrite += r.usage.CacheWrite
		row.Cost += r.usage.Cost
		priced = priced && r.usage.Priced
	}
	row.Priced = known > 0 && priced
	row.Unknown = known == 0
	row.Incomplete = !row.Unknown && known < len(t.replies)
	return row
}

// copyUsage copies the token counters and scope of u into row.
func copyUsage(row *api.UsageTurn, u *api.ChatUsage) {
	row.Input, row.Output = u.Input, u.Output
	row.CacheRead, row.CacheWrite = u.CacheRead, u.CacheWrite
	row.Scope = cut(u.Scope)
}

// usageRow returns just the counters of u, for attribution to a model.
func usageRow(u *api.ChatUsage) api.UsageTurn {
	var row api.UsageTurn
	copyUsage(&row, u)
	return row
}

// modelTotal adds up one model's share.
type modelTotal struct {
	model   api.UsageModel
	priced  int
	replies int
}

// addModel adds row's counters to model's share.
func addModel(models map[string]*modelTotal, model string, row api.UsageTurn, priced bool) *modelTotal {
	model = cut(model)
	if model == "" {
		model = "unknown"
	}
	m := models[model]
	if m == nil {
		m = &modelTotal{model: api.UsageModel{Model: model}}
		models[model] = m
	}
	m.model.Input += row.Input
	m.model.Output += row.Output
	m.model.CacheRead += row.CacheRead
	m.model.CacheWrite += row.CacheWrite
	m.replies++
	if priced {
		m.priced++
	}
	return m
}

// sortedModels returns the model shares, biggest first, with a stable tie
// break on the name.
func sortedModels(models map[string]*modelTotal) []api.UsageModel {
	out := make([]api.UsageModel, 0, len(models))
	for _, m := range models {
		m.model.Priced = m.replies > 0 && m.priced == m.replies
		out = append(out, m.model)
	}
	slices.SortFunc(out, func(a, b api.UsageModel) int {
		if d := (b.Input + b.Output) - (a.Input + a.Output); d != 0 {
			return d
		}
		return strings.Compare(a.Model, b.Model)
	})
	return out
}

// notes explains the limits of what the records prove.
func notes(p *parsed, st api.UsageState, partial bool) []string {
	var out []string
	if p.runtime {
		out = append(out, "Subagent usage is not reported by this runtime.")
	}
	if st.Unknown > 0 {
		out = append(out, turnCount(st.Unknown)+hasNoUsage(st.Unknown)+"; totals are a lower bound.")
	}
	if st.Incomplete > 0 {
		out = append(out, turnCount(st.Incomplete)+" stopped before all usage was reported.")
	}
	switch {
	case !st.Totals.Priced && (st.Totals.Input+st.Totals.Output+st.Totals.CacheRead+st.Totals.CacheWrite) > 0:
		out = append(out, "Snow has no price for this model; the cost is unknown.")
	case partial:
		out = append(out, "Some replies have no price; the cost is a lower bound.")
	}
	switch {
	case p.runtime && st.Totals.Priced:
		out = append(out, "Cost is an API-equivalent session estimate, not a subscription bill.")
		out = append(out, "Per-model cost is unavailable: the runtime reports one session estimate.")
	case st.Totals.Priced:
		out = append(out, "Cost is an estimate at list prices, not your provider bill.")
	}
	return out
}

// turnCount names n turns.
func turnCount(n int) string {
	if n == 1 {
		return "1 turn"
	}
	return fmt.Sprintf("%d turns", n)
}

// hasNoUsage agrees with turnCount.
func hasNoUsage(n int) string {
	if n == 1 {
		return " has no reported usage"
	}
	return " have no reported usage"
}

// hasTokens reports whether u counted any tokens at all.
func hasTokens(u *api.ChatUsage) bool {
	return u.Input+u.Output+u.CacheRead+u.CacheWrite > 0
}

// modelOfReply names the model one direct-API reply used. The vendor's
// reported ID wins, because that is what it billed.
func modelOfReply(r reply) string {
	if r.model != "" {
		return r.model
	}
	return r.requested
}

// modelOfTurn names the model one runtime turn used.
func modelOfTurn(p *parsed, t *turn) string {
	if t.usage != nil && t.usage.Requested != "" {
		return t.usage.Requested
	}
	if t.usage != nil && t.usage.Reported != "" {
		return t.usage.Reported
	}
	if t.model != "" {
		return t.model
	}
	return p.model
}

// cut bounds a name so any state fits a frame.
func cut(s string) string {
	if len(s) <= api.MaxUsageName {
		return s
	}
	return s[:api.MaxUsageName]
}

// title returns the conversation's title, including a runtime's.
func title(loaded *convstore.Conversation) string {
	if t := loaded.Title(); t != "" {
		return cut(t)
	}
	last := ""
	for _, rec := range loaded.Records {
		if r, ok := rec.(*convstore.RuntimeEvent); ok && r.Title != "" {
			last = r.Title
		}
	}
	return cut(last)
}

// parsed is a conversation read as turns of replies and tool calls.
type parsed struct {
	runtime bool
	model   string // the latest model the conversation asked for
	turns   []*turn
}

// turn is one user turn.
type turn struct {
	number      int
	at          time.Time
	model       string         // the turn's own model, when the source names one
	usage       *api.ChatUsage // a runtime's one snapshot for the turn
	replies     []reply
	calls       []*call
	interrupted bool
	closed      bool
}

// reply is one model request and its usage.
type reply struct {
	model     string
	requested string
	usage     *api.ChatUsage
}

// call is a tool call the agent made, with its result when one was saved.
type call struct {
	name   string
	input  json.RawMessage
	result string
	done   bool
}

// parse reads the records of a conversation into turns.
func parse(loaded *convstore.Conversation) *parsed {
	p := &parsed{runtime: loaded.Meta.Runtime != nil, model: loaded.Meta.Model}
	pending := map[string]*call{}
	var cur *turn

	start := func(at time.Time, number int) *turn {
		if number <= 0 {
			number = len(p.turns) + 1
		}
		if n := len(p.turns); n > 0 && number <= p.turns[n-1].number {
			number = p.turns[n-1].number + 1
		}
		t := &turn{number: number, at: at}
		p.turns = append(p.turns, t)
		return t
	}
	takeCall := func(t *turn, tool *api.ChatTool) {
		if t == nil || tool == nil {
			return
		}
		c := &call{name: tool.Name, input: tool.Input}
		t.calls = append(t.calls, c)
		pending[tool.ID] = c
	}
	doneCall := func(id, result string) {
		if c := pending[id]; c != nil {
			c.result, c.done = result, true
			delete(pending, id)
		}
	}

	for _, rec := range loaded.Records {
		switch r := rec.(type) {
		case *convstore.User:
			cur = start(r.At, 0)
		case *convstore.Assistant:
			if cur == nil {
				cur = start(r.At, 0)
			}
			rep := reply{model: r.Model, requested: p.model}
			if r.Usage != nil {
				u := &api.ChatUsage{Requested: p.model, Reported: r.Model,
					Input: r.Usage.Input, Output: r.Usage.Output,
					CacheRead: r.Usage.CacheRead, CacheWrite: r.Usage.CacheWrite}
				u.Cost, u.Priced = price(p.model, r.Usage)
				rep.usage = u
			}
			cur.replies = append(cur.replies, rep)
			for _, b := range r.Blocks {
				if b.Kind == convstore.BlockToolUse {
					takeCall(cur, &api.ChatTool{ID: b.ID, Name: b.Name, Input: b.Input})
				}
			}
		case *convstore.ToolResult:
			doneCall(r.ToolUseID, r.Content)
		case *convstore.Interruption:
			if cur != nil {
				cur.interrupted = true
			}
		case *convstore.ModelSwitch:
			p.model = r.Model
		case *convstore.RuntimeEvent:
			var e api.ChatEvent
			if json.Unmarshal(r.Display, &e) != nil {
				continue
			}
			switch e.Kind {
			case api.ChatEventTurnStarted:
				cur = start(r.At, e.Turn)
				cur.model = e.Model
			case api.ChatEventUsage:
				if cur == nil {
					cur = start(r.At, 0)
				}
				if e.Usage != nil {
					u := *e.Usage
					cur.usage = &u
					if u.Requested != "" {
						p.model = u.Requested
					}
				}
			case api.ChatEventModel:
				if e.Model != "" {
					p.model = e.Model
					if cur != nil {
						cur.model = e.Model
					}
				}
			case api.ChatEventToolCall:
				takeCall(cur, e.Tool)
			case api.ChatEventToolResult:
				if e.Tool != nil {
					doneCall(e.Tool.ID, e.Tool.Result)
				}
			case api.ChatEventTurnEnded:
				if cur != nil {
					cur.closed = true
					if e.Reason != api.ChatEndDone {
						cur.interrupted = true
					}
				}
			}
		}
	}

	// A file that ends mid-turn lost the rest of its usage.
	if n := len(p.turns); n > 0 {
		last := p.turns[n-1]
		if p.runtime {
			if !last.closed {
				last.interrupted = true
			}
		} else if loaded.MidTurn {
			last.interrupted = true
		}
	}
	return p
}

// price estimates the cost of one direct-API request.
func price(requested string, u *convstore.Usage) (float64, bool) {
	name, err := provider.ParseModelName(requested)
	if err != nil {
		return 0, false
	}
	return provider.Cost(name, provider.Usage{
		Input: u.Input, Output: u.Output, CacheRead: u.CacheRead, CacheWrite: u.CacheWrite,
	})
}
