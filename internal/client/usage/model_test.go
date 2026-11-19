package usage

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// clock is the time the test state's turns happened.
func clock(hour, minute int) time.Time {
	return time.Date(2026, 11, 17, hour, minute, 0, 0, time.UTC)
}

// state is a runtime conversation with a priced session estimate, one
// unknown turn, and a partial turn.
func state() api.UsageState {
	return api.UsageState{
		Folder:       "/home/u/snow",
		Conversation: "conv-7",
		Title:        "Fix the parser",
		Runtime:      "claude-code",
		Model:        "claude-sonnet-4-5",
		Turns:        4,
		Unknown:      1,
		Incomplete:   1,
		Totals: api.UsageTotals{
			Input: 12_300, Output: 900, CacheRead: 48_000, CacheWrite: 2_100,
			Cost: 0.42, Priced: true, Cumulative: true, LowerBound: true,
		},
		Models: []api.UsageModel{
			{Model: "claude-sonnet-4-5", Input: 12_300, Output: 900, CacheRead: 48_000, CacheWrite: 2_100},
		},
		Rows: []api.UsageTurn{
			{Turn: 4, At: clock(14, 2), Model: "claude-sonnet-4-5", Unknown: true, Interrupted: true, Scope: "main-agent"},
			{Turn: 3, At: clock(13, 58), Model: "claude-sonnet-4-5", Input: 4_200, Output: 300, Scope: "main-agent"},
			{Turn: 2, At: clock(13, 40), Model: "claude-sonnet-4-5", Input: 5_100, Output: 400, Incomplete: true, Scope: "main-agent"},
			{Turn: 1, At: clock(13, 12), Model: "claude-sonnet-4-5", Input: 3_000, Output: 200, Scope: "main-agent"},
		},
		Notes: []string{
			"Subagent usage is not reported by this runtime.",
			"1 turn has no reported usage; totals are a lower bound.",
			"1 turn stopped before all usage was reported.",
			"Cost is an API-equivalent session estimate, not a subscription bill.",
			"Per-model cost is unavailable: the runtime reports one session estimate.",
		},
	}
}

// TestUsagePaneGolden draws the pane at both sizes and in a narrow split,
// where a note wraps instead of losing its end.
func TestUsagePaneGolden(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 40}, {50, 30}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			m := New(7, theme.Default())
			m.SetSize(size[0], size[1])
			m.SetState(state())
			golden.RequireEqual(t, strings.Join(m.View(), "\n"))
		})
	}
}

// TestUsageEmpty: a pane with no conversation says what to do.
func TestUsageEmpty(t *testing.T) {
	m := New(7, theme.Default())
	m.SetSize(60, 12)
	m.SetState(api.UsageState{})
	got := strings.Join(m.View(), "\n")
	if !strings.Contains(got, "No conversation yet") {
		t.Fatalf("view: %q", got)
	}
}

// TestUsageRefreshKey sends a refresh action to the server.
func TestUsageRefreshKey(t *testing.T) {
	m := New(7, theme.Default())
	m.SetSize(60, 12)
	m.SetState(state())
	msg, ok := m.Key(tea.Key{Text: "r"}).(*api.SidePaneAction)
	if !ok || msg.Pane != 7 || msg.Action != api.UsageActRefresh {
		t.Fatalf("refresh key sent %+v", msg)
	}
}

// TestUsageScrollStaysInside: the keys and the wheel move the rows, but
// never past the first or last one.
func TestUsageScrollStaysInside(t *testing.T) {
	m := New(7, theme.Default())
	m.SetSize(60, 6)
	m.SetState(state())
	last := max(len(m.lines())-m.BodyHeight(), 0)
	m.Key(tea.Key{Code: tea.KeyEnd})
	if m.Top != last {
		t.Fatalf("top %d after End, want %d", m.Top, last)
	}
	m.Scroll(1) // one notch toward older rows
	if want := max(last-1, 0); m.Top != want {
		t.Fatalf("top %d after one wheel notch, want %d", m.Top, want)
	}
	m.Scroll(-100)
	if m.Top != last {
		t.Fatalf("top %d after scrolling past the end, want %d", m.Top, last)
	}
	m.Key(tea.Key{Code: tea.KeyHome})
	for range 40 {
		m.Key(tea.Key{Code: tea.KeyUp})
	}
	if m.Top != 0 {
		t.Fatalf("top %d after scrolling past the start, want 0", m.Top)
	}
}
