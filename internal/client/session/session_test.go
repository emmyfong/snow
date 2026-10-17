package session

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/emmyfong/snow/internal/client/theme"

	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/pkg/api"
)

// fixedClock keeps the status bar's time stable in goldens.
func fixedClock() time.Time { return time.Date(2026, 10, 15, 14, 2, 0, 0, time.UTC) }

// screen returns a session screen of cols by rows with a fixed clock.
func screen(cols, rows int) *Model {
	m := New(func(api.Message) {})
	m.now = fixedClock
	m.SetSize(cols, rows)
	return m
}

// attached returns a session screen that received a layout and a full screen.
// The pane is one row shorter than the screen: the status bar takes the last.
func attached(t *testing.T, cols, rows int) *Model {
	t.Helper()
	m := screen(cols, rows)
	m.Update(&api.Layout{Session: "work", Active: 1, Windows: []api.WindowInfo{{
		Index: 1, Name: "PowerShell",
		Panes: []api.PaneInfo{{ID: 1, Rect: api.Rect{W: cols, H: rows - 1}, Profile: "PowerShell", Focused: true}},
	}}})
	lines := []api.LineUpdate{
		{Row: 0, Text: "Windows PowerShell"},
		{Row: 1, Text: "PS C:\\Users\\emmy> \x1b[33mGet-ChildItem\x1b[0m"},
		{Row: 2, Text: "漢字 and emoji 😀 keep their width"},
	}
	for r := 3; r < rows-1; r++ {
		lines = append(lines, api.LineUpdate{Row: r, Text: fmt.Sprintf("line %d", r)})
	}
	m.Update(&api.PaneUpdate{Pane: 1, Lines: lines, Cursor: &api.Cursor{X: 18, Y: 1, Visible: true}})
	return m
}

func TestViewGolden80x24(t *testing.T)  { golden.RequireEqual(t, attached(t, 80, 24).View().Content) }
func TestViewGolden120x40(t *testing.T) { golden.RequireEqual(t, attached(t, 120, 40).View().Content) }

func TestPaneUpdateReplacesOnlySentRows(t *testing.T) {
	m := attached(t, 80, 24)
	m.Update(&api.PaneUpdate{Pane: 1, Lines: []api.LineUpdate{{Row: 3, Text: "changed"}}})
	rows := strings.Split(m.View().Content, "\n")
	if !strings.HasPrefix(rows[3], "changed") || !strings.HasPrefix(rows[4], "line 4") {
		t.Fatalf("rows 3 and 4 = %q, %q", rows[3], rows[4])
	}
}

func TestCursorFollowsUpdates(t *testing.T) {
	m := attached(t, 80, 24)
	c := m.View().Cursor
	if c == nil || c.X != 18 || c.Y != 1 {
		t.Fatalf("cursor %+v, want 18,1", c)
	}
	m.Update(&api.PaneUpdate{Pane: 1, Cursor: &api.Cursor{X: 0, Y: 2, Visible: false}})
	if m.View().Cursor != nil {
		t.Fatal("hidden cursor still shown")
	}
}

func TestUpdatesForOtherPanesIgnored(t *testing.T) {
	m := attached(t, 80, 24)
	before := m.View().Content
	m.Update(&api.PaneUpdate{Pane: 99, Lines: []api.LineUpdate{{Row: 0, Text: "not mine"}}})
	if m.View().Content != before {
		t.Fatal("an update for another pane changed the screen")
	}
}

// TestColorProfileAdaptsTheTheme: a 16-color terminal gets a theme whose text
// stays readable after its colors are reduced.
func TestColorProfileAdaptsTheTheme(t *testing.T) {
	m := screen(80, 24)
	m.Update(tea.ColorProfileMsg{Profile: colorprofile.ANSI})
	if want := theme.Default().For(colorprofile.ANSI); m.theme.TabActive != want.TabActive || m.theme.PromptFG != want.PromptFG {
		t.Fatal("the session screen ignored the terminal's color profile")
	}
}
