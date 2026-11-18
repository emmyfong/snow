package context

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// states are the context screens the goldens pin: a full observable
// breakdown from the direct-API path, and a partial one where a runtime
// reports no occupancy.
func states() map[string]api.ContextState {
	full := api.ContextState{
		Folder:       "/home/u/snow",
		Conversation: "conv-7",
		Title:        "Fix the parser",
		Model:        "anthropic/claude-sonnet-4-5",
		Occupancy: api.ContextOccupancy{
			Known: true, Tokens: 34_200, Window: 200_000,
			Source: "the last prompt of the direct-API reply",
		},
		Sources: []api.ContextSource{
			{Path: "internal/usage/context.go", Reads: 3, FirstTurn: 1, LastTurn: 4},
			{Path: "internal/server/usagepane.go", Reads: 2, Changed: true, FirstTurn: 2, LastTurn: 5},
			{Path: "pkg/api/contextpane.go", Reads: 1, FirstTurn: 5},
		},
		Reads: []api.ContextRead{
			{Path: "internal/usage/context.go", Range: "1-40", Reads: 3, Same: 2, Result: "unchanged",
				Evidence: "tool result text", FirstTurn: 1, LastTurn: 4},
			{Path: "internal/server/usagepane.go", Reads: 2, Changed: 1, Result: "changed",
				Evidence: "tool result text", FirstTurn: 2, LastTurn: 5},
		},
		Notes: []string{
			"Per-source token counts are unavailable: tool calls do not report the tokens their content added.",
			"Repeated reads compare tool result text, not a saved file version. A file change can justify a second read.",
		},
	}
	partial := api.ContextState{
		Folder:       "/home/u/snow",
		Conversation: "conv-8",
		Title:        "Ship the fix",
		Runtime:      "claude-code",
		Model:        "claude-sonnet-4-5",
		Occupancy: api.ContextOccupancy{
			Reason: "This runtime does not report how much of its context window it used. Snow does not estimate the hidden prompt.",
		},
		Sources: []api.ContextSource{
			{Path: "internal/usage/usage.go", Reads: 2, FirstTurn: 1, LastTurn: 2},
			{Path: "internal/server/convpane.go", Changed: true, FirstTurn: 2, LastTurn: 2},
		},
		Reads: []api.ContextRead{
			{Path: "internal/usage/usage.go", Reads: 2, Same: 1, Result: "unchanged",
				Evidence: "tool result text", FirstTurn: 1, LastTurn: 2},
		},
		Notes: []string{
			"This runtime does not report context occupancy or the hidden prompt's content.",
			"Per-source token counts are unavailable: tool calls do not report the tokens their content added.",
			"Repeated reads compare tool result text, not a saved file version. A file change can justify a second read.",
		},
	}
	return map[string]api.ContextState{"full": full, "partial": partial}
}

// TestContextPaneGolden draws each context state at both sizes.
func TestContextPaneGolden(t *testing.T) {
	for name, st := range states() {
		if err := st.Validate(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, size := range [][2]int{{80, 24}, {120, 40}} {
			t.Run(fmt.Sprintf("%s_%dx%d", name, size[0], size[1]), func(t *testing.T) {
				m := New(7, theme.Default())
				m.SetSize(size[0], size[1])
				m.SetState(st)
				golden.RequireEqual(t, strings.Join(m.View(), "\n"))
			})
		}
	}
}

// TestContextEmpty: a pane with no conversation says what to do.
func TestContextEmpty(t *testing.T) {
	m := New(7, theme.Default())
	m.SetSize(60, 12)
	m.SetState(api.ContextState{})
	got := strings.Join(m.View(), "\n")
	if !strings.Contains(got, "No conversation yet") {
		t.Fatalf("view: %q", got)
	}
}

// TestContextRefreshKey sends a refresh action to the server.
func TestContextRefreshKey(t *testing.T) {
	m := New(7, theme.Default())
	m.SetSize(60, 12)
	m.SetState(states()["full"])
	msg, ok := m.Key(tea.Key{Text: "r"}).(*api.SidePaneAction)
	if !ok || msg.Pane != 7 || msg.Action != api.ContextActRefresh {
		t.Fatalf("refresh key sent %+v", msg)
	}
}
