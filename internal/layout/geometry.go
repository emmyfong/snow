package layout

import "math"

// edgeEpsilon keeps floating-point error from pulling an edge one cell down,
// for example when 30 * (1/3) + 0.5 comes out just under 10.5.
const edgeEpsilon = 1e-9

// Rect is a cell rectangle; X and Y are the top-left corner.
type Rect struct {
	X, Y, W, H int
}

// Geometry is a tree laid out in a window. Panes maps each visible pane to
// its rectangle. Borders are the one-cell lines between siblings. Together
// they cover the window exactly, with no overlap.
type Geometry struct {
	Panes   map[PaneID]Rect
	Borders []Rect
}

// Rects lays tree t out in a window of cols by rows cells. Each split puts a
// one-cell border between its children and divides the rest by ratio. Edges
// round half up, as in tmux, so the first child of an odd split gets the
// extra cell. A zoomed pane fills the window alone.
//
// A window too small for its borders gives some panes zero width or height.
// Rects never returns a negative size or a rectangle outside the window.
func Rects(t *Tree, cols, rows int) Geometry {
	win := Rect{W: max(cols, 0), H: max(rows, 0)}
	if t.Zoomed != 0 {
		return Geometry{Panes: map[PaneID]Rect{t.Zoomed: win}}
	}
	return place(t.Root, win)
}

// place lays out the tree under root in win, ignoring zoom.
func place(root *Node, win Rect) Geometry {
	g := Geometry{Panes: map[PaneID]Rect{}}
	var walk func(*Node, Rect)
	walk = func(n *Node, r Rect) {
		if n.leaf() {
			g.Panes[n.Pane] = r
			return
		}
		across := n.Axis == Horizontal
		length := r.H
		if across {
			length = r.W
		}
		sizes := divide(length, n.Ratios)
		pos := 0
		for i, c := range n.Children {
			walk(c, r.slice(across, pos, sizes[i]))
			pos += sizes[i]
			if i == len(n.Children)-1 {
				break
			}
			if b := r.slice(across, pos, 1); b.W > 0 && b.H > 0 {
				g.Borders = append(g.Borders, b)
			}
			pos++
		}
	}
	if root != nil {
		walk(root, win)
	}
	return g
}

// divide splits length cells, minus one border between each pair of
// children, by ratios. Edges are rounded cumulatively, so the sizes always
// add up to the space available.
func divide(length int, ratios []float64) []int {
	n := len(ratios)
	avail := max(length-(n-1), 0)
	sizes := make([]int, n)
	var cum float64
	prev := 0
	for i, r := range ratios {
		cum += r
		edge := avail
		if i < n-1 {
			edge = min(int(math.Floor(float64(avail)*cum+0.5+edgeEpsilon)), avail)
		}
		sizes[i] = edge - prev
		prev = edge
	}
	return sizes
}

// slice returns the part of r from offset off to off+size along the
// horizontal axis when across is true, else the vertical one. It clips the
// result to r, so a border that does not fit has zero size.
func (r Rect) slice(across bool, off, size int) Rect {
	if across {
		x0 := min(r.X+off, r.X+r.W)
		x1 := min(r.X+off+size, r.X+r.W)
		return Rect{X: x0, Y: r.Y, W: x1 - x0, H: r.H}
	}
	y0 := min(r.Y+off, r.Y+r.H)
	y1 := min(r.Y+off+size, r.Y+r.H)
	return Rect{X: r.X, Y: y0, W: r.W, H: y1 - y0}
}
