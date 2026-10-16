package mouse

import "github.com/emmyfong/snow/pkg/api"

// Kind is what lies under a cell.
type Kind int

// What a cell can hold.
const (
	None   Kind = iota // blank space beyond the panes, or off the screen
	Pane               // a pane's cell
	Border             // a line between panes
	Tab                // a window's tab in the status bar
	Bar                // the status bar outside the tabs
)

// Axis is the direction a border runs.
type Axis int

// Border directions.
const (
	NoAxis     Axis = iota
	Vertical        // a line between panes side by side; dragging moves it left or right
	Horizontal      // a line between panes stacked; dragging moves it up or down
)

// Hit is the result of HitTest.
type Hit struct {
	Kind Kind
	// Pane is the pane ID, and X and Y the cell inside it, for Kind Pane.
	Pane int
	X, Y int
	// Window is the window index, for Kind Tab.
	Window int
	// Axis, Before, and After describe a Border. Before is the pane on its
	// left or top side, After the one on its right or bottom side. A border
	// cell that no straight border reaches has NoAxis.
	Axis          Axis
	Before, After int
}

// TabSpan is the columns a window's tab covers in the status bar.
type TabSpan struct {
	Index int // the window index
	X, W  int
}

// HitTest says what lies under cell x, y of a screen of cols by rows cells.
// The last row is the status bar; panes cover the rows above it. Cells in
// the panes' area that no pane covers are borders. A client larger than
// the session leaves blank cells beyond the panes, which are None.
func HitTest(panes []api.PaneInfo, tabs []TabSpan, x, y, cols, rows int) Hit {
	if x < 0 || y < 0 || x >= cols || y >= rows {
		return Hit{Kind: None}
	}
	if y == rows-1 {
		for _, t := range tabs {
			if x >= t.X && x < t.X+t.W {
				return Hit{Kind: Tab, Window: t.Index}
			}
		}
		return Hit{Kind: Bar}
	}
	g := newGrid(panes)
	if id := g.at(x, y); id > 0 {
		for _, p := range panes {
			if p.ID == id {
				return Hit{Kind: Pane, Pane: id, X: x - p.Rect.X, Y: y - p.Rect.Y}
			}
		}
	}
	if !g.inArea(x, y) {
		return Hit{Kind: None}
	}
	return g.border(x, y)
}

// grid answers which pane covers a cell.
type grid struct {
	panes        []api.PaneInfo
	areaW, areaH int
}

func newGrid(panes []api.PaneInfo) grid {
	g := grid{panes: panes}
	for _, p := range panes {
		g.areaW, g.areaH = max(g.areaW, p.Rect.X+p.Rect.W), max(g.areaH, p.Rect.Y+p.Rect.H)
	}
	return g
}

func (g grid) inArea(x, y int) bool { return x >= 0 && y >= 0 && x < g.areaW && y < g.areaH }

// at returns the ID of the pane covering x, y, or 0.
func (g grid) at(x, y int) int {
	for _, p := range g.panes {
		r := p.Rect
		if x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H {
			return p.ID
		}
	}
	return 0
}

// border describes the border cell x, y by the panes on its two sides. A
// junction cell has a pane on one side only, so the search walks along the
// line it sits on until it finds a cell with panes on both sides. A vertical
// line wins over a horizontal one at a crossing.
func (g grid) border(x, y int) Hit {
	if h, ok := g.straight(x, y); ok {
		return h
	}
	for d := 1; d < max(g.areaW, g.areaH); d++ {
		for _, step := range [][2]int{{0, -d}, {0, d}} {
			if h, ok := g.straight(x+step[0], y+step[1]); ok && h.Axis == Vertical {
				return h
			}
		}
	}
	for d := 1; d < max(g.areaW, g.areaH); d++ {
		for _, step := range [][2]int{{-d, 0}, {d, 0}} {
			if h, ok := g.straight(x+step[0], y+step[1]); ok && h.Axis == Horizontal {
				return h
			}
		}
	}
	return Hit{Kind: Border}
}

// straight reports the border at x, y when panes touch it on opposite sides.
func (g grid) straight(x, y int) (Hit, bool) {
	if !g.inArea(x, y) || g.at(x, y) > 0 {
		return Hit{}, false
	}
	if l, r := g.at(x-1, y), g.at(x+1, y); l > 0 && r > 0 {
		return Hit{Kind: Border, Axis: Vertical, Before: l, After: r}, true
	}
	if u, d := g.at(x, y-1), g.at(x, y+1); u > 0 && d > 0 {
		return Hit{Kind: Border, Axis: Horizontal, Before: u, After: d}, true
	}
	return Hit{}, false
}
