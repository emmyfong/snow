package session

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// TestStatusBarMostUrgentState: a window with two agents shows the more
// urgent state. The server sends it as the window's state, which counts
// panes a zoom hides; the tab shows that, not the shown panes'.
func TestStatusBarMostUrgentState(t *testing.T) {
	tests := []struct {
		name   string
		window string
		panes  [2]string
		want   ui.Mark
	}{
		{"needs you over idle", api.AgentNeedsYou, [2]string{api.AgentIdle, api.AgentNeedsYou}, ui.MarkNeedsYou},
		{"a hidden pane needs you", api.AgentNeedsYou, [2]string{api.AgentWorking, ""}, ui.MarkNeedsYou},
		{"working over idle", api.AgentWorking, [2]string{api.AgentWorking, api.AgentIdle}, ui.MarkWorking},
		{"a shell shows nothing", "", [2]string{"", ""}, ui.MarkNone},
	}
	for _, tt := range tests {
		m := screen(80, 24)
		m.Update(&api.Layout{Session: "s", Active: 0, Windows: []api.WindowInfo{
			{Index: 0, Name: "sh", Panes: []api.PaneInfo{{ID: 9, Rect: api.Rect{W: 80, H: 23}, Focused: true}}},
			{Index: 1, Name: "agents", AgentState: tt.window, Panes: []api.PaneInfo{
				{ID: 1, Rect: api.Rect{W: 40, H: 23}, AgentState: tt.panes[0]},
				{ID: 2, Rect: api.Rect{X: 41, W: 39, H: 23}, AgentState: tt.panes[1]},
			}},
		}})
		tabs := m.statusOptions().Tabs
		if got := tabs[1].Agent; got != tt.want {
			t.Errorf("%s: mark %v, want %v", tt.name, got, tt.want)
		}
		if tabs[0].Agent != ui.MarkNone {
			t.Errorf("%s: the shell window has a mark", tt.name)
		}
	}
}

// TestStatusBarShowsAgentMark: the mark reaches the drawn bar.
func TestStatusBarShowsAgentMark(t *testing.T) {
	m := screen(80, 24)
	m.Update(&api.Layout{Session: "s", Active: 0, Windows: []api.WindowInfo{
		{Index: 0, Name: "claude", AgentState: api.AgentNeedsYou, Panes: []api.PaneInfo{{ID: 1, Rect: api.Rect{W: 80, H: 23}, Focused: true, AgentState: api.AgentNeedsYou}}},
	}})
	rows := strings.Split(m.View().Content, "\n")
	if bar := ansi.Strip(rows[len(rows)-1]); !strings.Contains(bar, "0 claude* ●") {
		t.Fatalf("status bar %q, want the needs-you mark", bar)
	}
}

// TestStatusBarShowsDrivenMark: the mark that an agent types into a window
// reaches the drawn bar.
func TestStatusBarShowsDrivenMark(t *testing.T) {
	m := screen(80, 24)
	m.Update(&api.Layout{Session: "s", Active: 0, Windows: []api.WindowInfo{
		{Index: 0, Name: "sh", Driven: true, Panes: []api.PaneInfo{{ID: 1, Rect: api.Rect{W: 80, H: 23}, Focused: true, Driven: true}}},
	}})
	rows := strings.Split(m.View().Content, "\n")
	if bar := ansi.Strip(rows[len(rows)-1]); !strings.Contains(bar, "0 sh* "+m.theme.Glyphs.Driven) {
		t.Fatalf("status bar %q, want the driven mark", bar)
	}
}

func goldenAgentMarks(t *testing.T, cols, rows int) {
	m := screen(cols, rows)
	p := []api.PaneInfo{{ID: 4, Rect: api.Rect{W: cols, H: rows - 1}, Profile: "Claude Code", Focused: true, AgentState: api.AgentWorking}}
	m.Update(&api.Layout{Session: "rename-fix", Active: 1, Windows: []api.WindowInfo{
		{Index: 0, Name: "agents", AgentState: api.AgentIdle},
		{Index: 1, Name: "planner", AgentState: api.AgentWorking, Panes: p},
		{Index: 2, Name: "builder", AgentState: api.AgentNeedsYou},
		{Index: 3, Name: "terminal"},
	}})
	fill(m, p)
	golden.RequireEqual(t, m.View().Content)
}

func TestAgentMarksGolden80x24(t *testing.T)  { goldenAgentMarks(t, 80, 24) }
func TestAgentMarksGolden120x40(t *testing.T) { goldenAgentMarks(t, 120, 40) }
