package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

// WindowPane is one pane to draw: its rectangle in cells, its rows, whether
// it has the focus, and whether an agent types into it.
type WindowPane struct {
	X, Y, W, H int
	Lines      []string
	Focused    bool
	Driven     bool
}

// WindowOptions configures Window.
type WindowOptions struct {
	Width, Height int
	Panes         []WindowPane
}

// Window draws panes into exactly Width by Height cells. The panes tile an
// area from the top-left corner; every cell of that area that no pane covers
// is a border, drawn with the junction its neighbors call for. Borders that
// touch the focused pane use the focus color, and borders of a pane an agent
// types into use the accent color, which wins over the focus color. Cells
// outside the area are blank, for a client larger than the session.
func Window(th theme.Theme, o WindowOptions) string {
	if o.Width <= 0 || o.Height <= 0 {
		return ""
	}
	g := newGrid(o)
	plain := lipgloss.NewStyle().Foreground(th.Border)
	focus := lipgloss.NewStyle().Foreground(th.FocusBorder)
	accent := lipgloss.NewStyle().Foreground(th.Accent)
	var b strings.Builder
	for y := range o.Height {
		if y > 0 {
			b.WriteByte('\n')
		}
		var row strings.Builder
		for x := 0; x < o.Width; {
			switch owner := g.owner(x, y); {
			case owner >= 0:
				p := o.Panes[owner]
				w := min(p.X+p.W, o.Width) - x
				row.WriteString(paneRow(p.Lines, y-p.Y, w))
				x += w
				continue
			case g.inArea(x, y):
				style := plain
				switch {
				case g.touchesDriven(x, y):
					style = accent
				case g.touchesFocus(x, y):
					style = focus
				}
				row.WriteString(style.Render(g.glyph(th.Glyphs, x, y)))
			default:
				row.WriteByte(' ')
			}
			x++
		}
		b.WriteString(ansi.Truncate(row.String(), o.Width, ""))
	}
	return b.String()
}

// grid maps each cell to the pane that covers it.
type grid struct {
	w, h         int
	areaW, areaH int
	cells        []int // pane index per cell, or -1
	focused      int
	driven       map[int]bool
}

func newGrid(o WindowOptions) *grid {
	g := &grid{w: o.Width, h: o.Height, cells: make([]int, o.Width*o.Height), focused: -1, driven: map[int]bool{}}
	for i := range g.cells {
		g.cells[i] = -1
	}
	for i, p := range o.Panes {
		g.areaW, g.areaH = max(g.areaW, p.X+p.W), max(g.areaH, p.Y+p.H)
		if p.Focused {
			g.focused = i
		}
		if p.Driven {
			g.driven[i] = true
		}
		for y := max(p.Y, 0); y < min(p.Y+p.H, o.Height); y++ {
			for x := max(p.X, 0); x < min(p.X+p.W, o.Width); x++ {
				g.cells[y*o.Width+x] = i
			}
		}
	}
	return g
}

func (g *grid) owner(x, y int) int {
	if x < 0 || y < 0 || x >= g.w || y >= g.h {
		return -2
	}
	return g.cells[y*g.w+x]
}

func (g *grid) inArea(x, y int) bool { return x >= 0 && y >= 0 && x < g.areaW && y < g.areaH }

func (g *grid) border(x, y int) bool { return g.inArea(x, y) && g.owner(x, y) == -1 }

func (g *grid) touchesFocus(x, y int) bool {
	if g.focused < 0 {
		return false
	}
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if g.owner(x+dx, y+dy) == g.focused {
				return true
			}
		}
	}
	return false
}

// touchesDriven reports whether a cell next to or in a pane an agent types
// into is a border, so that pane's borders show the accent color.
func (g *grid) touchesDriven(x, y int) bool {
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if i := g.owner(x+dx, y+dy); i >= 0 && g.driven[i] {
				return true
			}
		}
	}
	return false
}

// glyph picks the border character from which neighbors are borders too.
func (g *grid) glyph(gl theme.Glyphs, x, y int) string {
	up, down := g.border(x, y-1), g.border(x, y+1)
	left, right := g.border(x-1, y), g.border(x+1, y)
	switch {
	case up && down && left && right:
		return gl.Cross
	case up && down && right:
		return gl.TeeRight
	case up && down && left:
		return gl.TeeLeft
	case left && right && down:
		return gl.TeeDown
	case left && right && up:
		return gl.TeeUp
	case down && right:
		return gl.TopLeft
	case down && left:
		return gl.TopRight
	case up && right:
		return gl.BottomLeft
	case up && left:
		return gl.BottomRight
	case left || right:
		return gl.Horizontal
	default:
		return gl.Vertical
	}
}
