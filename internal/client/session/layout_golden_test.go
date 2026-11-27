package session

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/pkg/api"
)

// fill sends each pane rows that name it, so goldens show where it is drawn.
func fill(m *Model, panes []api.PaneInfo) {
	for _, p := range panes {
		var lines []api.LineUpdate
		for r := range p.Rect.H {
			lines = append(lines, api.LineUpdate{Row: r, Text: fmt.Sprintf("pane %d row %d", p.ID, r)})
		}
		m.Update(&api.PaneUpdate{Pane: p.ID, Lines: lines})
	}
}

// threePanes is h(1, v(2, 3)) laid out as the server does at cols by rows.
func threePanes(cols, rows int) []api.PaneInfo {
	area := rows - 3
	left := (cols - 1 + 1) / 2 // tmux rounding: the first child gets the extra cell
	top := (area - 1 + 1) / 2
	return []api.PaneInfo{
		{ID: 1, Rect: api.Rect{Y: 1, W: left, H: area}, Profile: "pwsh"},
		{ID: 2, Rect: api.Rect{X: left + 1, Y: 1, W: cols - left - 1, H: top}, Profile: "Ubuntu", Focused: true},
		{ID: 3, Rect: api.Rect{X: left + 1, Y: top + 2, W: cols - left - 1, H: area - top - 1}, Profile: "cmd"},
	}
}

func goldenThreePanes(t *testing.T, cols, rows int) {
	m := screen(cols, rows)
	panes := threePanes(cols, rows)
	m.Update(&api.Layout{Session: "api", Active: 1, Windows: []api.WindowInfo{{Index: 1, Name: "dev", Panes: panes}}})
	fill(m, panes)
	golden.RequireEqual(t, m.View().Content)
}

func goldenZoomed(t *testing.T, cols, rows int) {
	m := screen(cols, rows)
	p := []api.PaneInfo{{ID: 2, Rect: api.Rect{Y: 1, W: cols, H: rows - 3}, Profile: "Ubuntu", Focused: true}}
	m.Update(&api.Layout{Session: "api", Active: 1, Windows: []api.WindowInfo{{Index: 1, Name: "dev", Zoomed: true, Panes: p}}})
	fill(m, p)
	golden.RequireEqual(t, m.View().Content)
}

func goldenThreeWindows(t *testing.T, cols, rows int) {
	m := screen(cols, rows)
	p := []api.PaneInfo{{ID: 4, Rect: api.Rect{Y: 1, W: cols, H: rows - 3}, Profile: "cmd", Focused: true}}
	m.Update(&api.Layout{Session: "api", Active: 2, Windows: []api.WindowInfo{
		{Index: 1, Name: "dev"},
		{Index: 2, Name: "logs", Panes: p},
		{Index: 3, Name: "pwsh"},
	}})
	fill(m, p)
	golden.RequireEqual(t, m.View().Content)
}

func TestThreePanesGolden80x24(t *testing.T)    { goldenThreePanes(t, 80, 24) }
func TestThreePanesGolden120x40(t *testing.T)   { goldenThreePanes(t, 120, 40) }
func TestZoomedGolden80x24(t *testing.T)        { goldenZoomed(t, 80, 24) }
func TestZoomedGolden120x40(t *testing.T)       { goldenZoomed(t, 120, 40) }
func TestThreeWindowsGolden80x24(t *testing.T)  { goldenThreeWindows(t, 80, 24) }
func TestThreeWindowsGolden120x40(t *testing.T) { goldenThreeWindows(t, 120, 40) }

func goldenSidePaneHeader(t *testing.T, cols, rows int) {
	m := screen(cols, rows)
	p := []api.PaneInfo{{
		ID: 1, Kind: api.PaneGit, Rect: api.Rect{Y: 1, W: cols, H: rows - 3},
		Profile: "Claude Code (Ubuntu)", Folder: "/home/user/repo", Focused: true,
	}}
	m.Update(&api.Layout{Session: "work", Active: 1, Windows: []api.WindowInfo{{Index: 1, Name: "dev", Panes: p}}})
	golden.RequireEqual(t, m.View().Content)
}

func TestSidePaneHeaderGolden80x24(t *testing.T)  { goldenSidePaneHeader(t, 80, 24) }
func TestSidePaneHeaderGolden120x40(t *testing.T) { goldenSidePaneHeader(t, 120, 40) }

func TestSidePaneHeaderUsesKindAndKeepsSourceProfile(t *testing.T) {
	m := screen(80, 24)
	pane := api.PaneInfo{ID: 1, Kind: api.PaneGit, Profile: "Claude Code (Ubuntu)", Folder: "/home/user/repo", Focused: true}
	m.Update(&api.Layout{Session: "work", Active: 1, Windows: []api.WindowInfo{{Index: 1, Panes: []api.PaneInfo{pane}}}})
	if got := m.windowPanes()[0].Name; got != api.PaneGit {
		t.Fatalf("side pane header name %q, want %q", got, api.PaneGit)
	}
	m.openPicker("Split", api.Command{Op: api.OpSplit, Args: map[string]string{}})
	if got := m.picker.Choice(); got != pane.Profile {
		t.Fatalf("split picker profile %q, want source profile %q", got, pane.Profile)
	}
}

func TestWindowTabsAbovePanes(t *testing.T) {
	m := screen(80, 24)
	p := []api.PaneInfo{{ID: 1, Rect: api.Rect{W: 80, H: 22}, Focused: true}}
	m.Update(&api.Layout{Session: "api", Active: 1, Windows: []api.WindowInfo{{Index: 1, Name: "dev", Panes: p}}})
	rows := strings.Split(m.View().Content, "\n")
	if !strings.Contains(rows[0], "1 dev*") {
		t.Fatalf("window bar does not show the tab on row 0: %q", rows[0])
	}
	if strings.Contains(rows[len(rows)-1], "1 dev*") {
		t.Fatalf("status line still shows the tab: %q", rows[len(rows)-1])
	}
}

// goldenCloseMark shows the focused bottom-right pane's top controls.
func goldenCloseMark(t *testing.T, cols, rows int) {
	m := screen(cols, rows)
	panes := threePanes(cols, rows)
	for i := range panes {
		panes[i].Focused = panes[i].ID == 3
	}
	m.Update(&api.Layout{Session: "api", Active: 1, Windows: []api.WindowInfo{{Index: 1, Name: "dev", Panes: panes}}})
	fill(m, panes)
	golden.RequireEqual(t, m.View().Content)
}

func TestCloseMarkGolden80x24(t *testing.T)  { goldenCloseMark(t, 80, 24) }
func TestCloseMarkGolden120x40(t *testing.T) { goldenCloseMark(t, 120, 40) }

// goldenCloseMarkTop shows the focused top pane's header and controls.
func goldenCloseMarkTop(t *testing.T, cols, rows int) {
	m := screen(cols, rows)
	area := rows - 3
	top := (area - 1 + 1) / 2
	panes := []api.PaneInfo{
		{ID: 1, Rect: api.Rect{Y: 1, W: cols, H: top}, Profile: "pwsh", Focused: true},
		{ID: 2, Rect: api.Rect{Y: top + 2, W: cols, H: area - top - 1}, Profile: "cmd"},
	}
	m.Update(&api.Layout{Session: "api", Active: 1, Windows: []api.WindowInfo{{Index: 1, Name: "dev", Panes: panes}}})
	fill(m, panes)
	golden.RequireEqual(t, m.View().Content)
}

func TestCloseMarkTopGolden80x24(t *testing.T)  { goldenCloseMarkTop(t, 80, 24) }
func TestCloseMarkTopGolden120x40(t *testing.T) { goldenCloseMarkTop(t, 120, 40) }
