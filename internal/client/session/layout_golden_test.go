package session

import (
	"fmt"
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
	area := rows - 1
	left := (cols - 1 + 1) / 2 // tmux rounding: the first child gets the extra cell
	top := (area - 1 + 1) / 2
	return []api.PaneInfo{
		{ID: 1, Rect: api.Rect{W: left, H: area}, Profile: "pwsh"},
		{ID: 2, Rect: api.Rect{X: left + 1, W: cols - left - 1, H: top}, Profile: "Ubuntu", Focused: true},
		{ID: 3, Rect: api.Rect{X: left + 1, Y: top + 1, W: cols - left - 1, H: area - top - 1}, Profile: "cmd"},
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
	p := []api.PaneInfo{{ID: 2, Rect: api.Rect{W: cols, H: rows - 1}, Profile: "Ubuntu", Focused: true}}
	m.Update(&api.Layout{Session: "api", Active: 1, Windows: []api.WindowInfo{{Index: 1, Name: "dev", Zoomed: true, Panes: p}}})
	fill(m, p)
	golden.RequireEqual(t, m.View().Content)
}

func goldenThreeWindows(t *testing.T, cols, rows int) {
	m := screen(cols, rows)
	p := []api.PaneInfo{{ID: 4, Rect: api.Rect{W: cols, H: rows - 1}, Profile: "cmd", Focused: true}}
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
