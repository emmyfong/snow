package api

import (
	"errors"
	"strings"
	"testing"
)

// TestContextStateValidate: a measured token count may exceed the price
// table's window, and that must not make the state undrawable. Malformed
// states still fail.
func TestContextStateValidate(t *testing.T) {
	over := ContextState{
		Title: "t", Folder: "/w", Conversation: "c", Model: "m", Runtime: "claude-code",
		Occupancy: ContextOccupancy{Known: true, Tokens: 500_000, Window: 200_000},
		Sources:   []ContextSource{{Path: "a.go", Reads: 2}},
		Reads: []ContextRead{
			{Path: "a.go", Range: "1-20", Reads: 2, Same: 1, Result: "unchanged"},
		},
		Notes: []string{"note"},
	}
	if err := over.Validate(); err != nil {
		t.Fatalf("measured tokens over a window: %v", err)
	}

	long := strings.Repeat("x", MaxContextName+1)
	tooManyReads := make([]ContextRead, MaxContextReads+1)
	tooManySources := make([]ContextSource, MaxContextSources+1)
	tests := map[string]ContextState{
		"long title":      {Title: long},
		"negative tokens": {Occupancy: ContextOccupancy{Tokens: -1}},
		"empty source":    {Sources: []ContextSource{{Reads: 1}}},
		"too many sources": {
			Sources: tooManySources,
		},
		"too many reads": {Reads: tooManyReads},
		"read count mismatch": {
			Reads: []ContextRead{{Path: "a", Reads: 3, Same: 1, Changed: 0, Result: "mixed"}},
		},
		"unknown result": {
			Reads: []ContextRead{{Path: "a", Reads: 2, Same: 1, Result: "wasted"}},
		},
	}
	for name, st := range tests {
		if err := st.Validate(); !errors.Is(err, ErrInvalidSidePane) {
			t.Errorf("%s.Validate() = %v, want ErrInvalidSidePane", name, err)
		}
	}
}
