package api

import (
	"fmt"
	"time"
)

// PaneUsage is the usage pane: what a conversation's observable usage adds
// up to, by turn and model. Its state is UsageState.
const PaneUsage = "usage"

// UsageActRefresh re-reads the conversation's records.
const UsageActRefresh = "refresh"

// Bounds on a usage pane state. A conversation with more turns than
// MaxUsageTurns shows the newest and says how many it left out.
const (
	MaxUsageTurns  = 200
	MaxUsageModels = 16
	MaxUsageNotes  = 8
	MaxUsageName   = 256
)

// UsageState is the state of a usage pane. It follows one conversation, so
// it rebuilds from that conversation's records after a restore.
type UsageState struct {
	Folder string `json:"folder,omitempty"`
	// Conversation is the conversation the pane follows.
	Conversation string `json:"conversation,omitempty"`
	Title        string `json:"title,omitempty"`
	// Runtime names the external runtime that reported the usage, or "" for
	// Snow's direct-API loop.
	Runtime string `json:"runtime,omitempty"`
	Model   string `json:"model,omitempty"` // the latest model the conversation asked for
	// Turns counts the conversation's user turns. Unknown counts those with
	// no reported usage, and Incomplete those whose reported usage is a
	// lower bound.
	Turns      int          `json:"turns"`
	Unknown    int          `json:"unknown,omitempty"`
	Incomplete int          `json:"incomplete,omitempty"`
	Totals     UsageTotals  `json:"totals"`
	Models     []UsageModel `json:"models,omitempty"`
	// Rows are the newest turns first, up to MaxUsageTurns. Earlier counts
	// the turns left out.
	Rows    []UsageTurn `json:"rows,omitempty"`
	Earlier int         `json:"earlier,omitempty"`
	// Notes explain a limit of the data: a lower bound, a missing price, or
	// a scope the runtime does not report.
	Notes   []string `json:"notes,omitempty"`
	Loading bool     `json:"loading,omitempty"`
	Error   string   `json:"error,omitempty"`
}

// UsageTotals adds up the usage the records prove.
type UsageTotals struct {
	Input      int `json:"input"`
	Output     int `json:"output"`
	CacheRead  int `json:"cacheRead,omitempty"`
	CacheWrite int `json:"cacheWrite,omitempty"`
	// Cost is an API-equivalent estimate in US dollars when Priced is set.
	// It is never an account balance.
	Cost   float64 `json:"cost,omitempty"`
	Priced bool    `json:"priced,omitempty"`
	// Cumulative means Cost is the runtime's session estimate: it replaces
	// the total and is not the sum of the rows.
	Cumulative bool `json:"cumulative,omitempty"`
	// LowerBound means some turns have no usage or an interrupted one, so
	// the totals are at least this much.
	LowerBound bool `json:"lowerBound,omitempty"`
}

// UsageModel adds up one model's usage. A cumulative cost cannot be split
// between models, so Priced is false for a runtime's models.
type UsageModel struct {
	Model      string  `json:"model"`
	Input      int     `json:"input"`
	Output     int     `json:"output"`
	CacheRead  int     `json:"cacheRead,omitempty"`
	CacheWrite int     `json:"cacheWrite,omitempty"`
	Cost       float64 `json:"cost,omitempty"`
	Priced     bool    `json:"priced,omitempty"`
}

// UsageTurn is one user turn's observable usage.
type UsageTurn struct {
	Turn       int       `json:"turn"`
	At         time.Time `json:"at"`
	Model      string    `json:"model,omitempty"`
	Input      int       `json:"input"`
	Output     int       `json:"output"`
	CacheRead  int       `json:"cacheRead,omitempty"`
	CacheWrite int       `json:"cacheWrite,omitempty"`
	// Cost is the turn's estimate when the source reports one per reply.
	// A runtime reports one session estimate instead, so its rows have none.
	Cost   float64 `json:"cost,omitempty"`
	Priced bool    `json:"priced,omitempty"`
	// Unknown means the turn reported no usage at all. Incomplete means it
	// reported some, but stopped before all of it arrived.
	Unknown     bool `json:"unknown,omitempty"`
	Incomplete  bool `json:"incomplete,omitempty"`
	Interrupted bool `json:"interrupted,omitempty"`
	// Scope names what the counters exclude, such as "main-agent" for a
	// runtime that leaves child agents out.
	Scope string `json:"scope,omitempty"`
}

// Kind implements SideState.
func (*UsageState) Kind() string { return PaneUsage }

// Validate checks that st can be drawn.
func (st *UsageState) Validate() error {
	for name, s := range map[string]string{
		"title": st.Title, "model": st.Model, "runtime": st.Runtime, "conversation": st.Conversation, "folder": st.Folder,
	} {
		if len(s) > MaxUsageName {
			return fmt.Errorf("%w: %s is %d bytes, at most %d", ErrInvalidSidePane, name, len(s), MaxUsageName)
		}
	}
	if st.Turns < 0 || st.Unknown < 0 || st.Incomplete < 0 || st.Unknown+st.Incomplete > st.Turns {
		return fmt.Errorf("%w: %d turns, %d unknown, %d incomplete", ErrInvalidSidePane, st.Turns, st.Unknown, st.Incomplete)
	}
	if len(st.Rows) > MaxUsageTurns {
		return fmt.Errorf("%w: %d rows, at most %d", ErrInvalidSidePane, len(st.Rows), MaxUsageTurns)
	}
	if len(st.Models) > MaxUsageModels {
		return fmt.Errorf("%w: %d models, at most %d", ErrInvalidSidePane, len(st.Models), MaxUsageModels)
	}
	if len(st.Notes) > MaxUsageNotes {
		return fmt.Errorf("%w: %d notes, at most %d", ErrInvalidSidePane, len(st.Notes), MaxUsageNotes)
	}
	for _, m := range st.Models {
		if m.Model == "" || len(m.Model) > MaxUsageName {
			return fmt.Errorf("%w: model %q", ErrInvalidSidePane, m.Model)
		}
		if m.Input < 0 || m.Output < 0 || m.CacheRead < 0 || m.CacheWrite < 0 || m.Cost < 0 {
			return fmt.Errorf("%w: model %q has a negative count", ErrInvalidSidePane, m.Model)
		}
	}
	seen := map[int]bool{}
	for _, r := range st.Rows {
		if r.Turn < 1 {
			return fmt.Errorf("%w: turn %d", ErrInvalidSidePane, r.Turn)
		}
		if seen[r.Turn] {
			return fmt.Errorf("%w: turn %d twice", ErrInvalidSidePane, r.Turn)
		}
		seen[r.Turn] = true
		if r.Input < 0 || r.Output < 0 || r.CacheRead < 0 || r.CacheWrite < 0 || r.Cost < 0 {
			return fmt.Errorf("%w: turn %d has a negative count", ErrInvalidSidePane, r.Turn)
		}
		if len(r.Model) > MaxUsageName || len(r.Scope) > MaxUsageName {
			return fmt.Errorf("%w: turn %d has a long name", ErrInvalidSidePane, r.Turn)
		}
	}
	if st.Totals.Input < 0 || st.Totals.Output < 0 || st.Totals.CacheRead < 0 || st.Totals.CacheWrite < 0 || st.Totals.Cost < 0 {
		return fmt.Errorf("%w: totals have a negative count", ErrInvalidSidePane)
	}
	return nil
}
