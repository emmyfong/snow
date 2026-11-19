package api

import (
	"errors"
	"strings"
	"testing"
)

// TestUsageStateValidate: a well-formed state draws, and a malformed one
// fails with ErrInvalidSidePane.
func TestUsageStateValidate(t *testing.T) {
	ok := UsageState{
		Title: "t", Folder: "/w", Conversation: "c", Model: "m", Runtime: "claude-code",
		Turns: 2, Unknown: 1, Incomplete: 1,
		Totals: UsageTotals{Input: 10, Output: 20, Cost: 0.1, Priced: true},
		Models: []UsageModel{{Model: "m", Input: 10}},
		Rows:   []UsageTurn{{Turn: 1, Input: 10}, {Turn: 2, Unknown: true}},
		Notes:  []string{"note"},
	}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid state: %v", err)
	}

	long := strings.Repeat("x", MaxUsageName+1)
	tests := map[string]UsageState{
		"long title":         {Title: long},
		"unknown past turns": {Turns: 1, Unknown: 1, Incomplete: 1},
		"negative totals":    {Totals: UsageTotals{Input: -1}},
		"empty model":        {Models: []UsageModel{{}}},
		"zero turn":          {Rows: []UsageTurn{{Turn: 0}}},
		"duplicate turn":     {Rows: []UsageTurn{{Turn: 1}, {Turn: 1}}},
		"too many rows":      {Rows: make([]UsageTurn, MaxUsageTurns+1)},
		"toomany models":     {Models: make([]UsageModel, MaxUsageModels+1)},
	}
	for name, st := range tests {
		if err := st.Validate(); !errors.Is(err, ErrInvalidSidePane) {
			t.Errorf("%s.Validate() = %v, want ErrInvalidSidePane", name, err)
		}
	}
}
