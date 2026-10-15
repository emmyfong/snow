package layout

import "fmt"

// Neighbor returns the pane next to pane id on side d, in a window of cols
// by rows cells. A neighbor sits just past the border on that side and
// overlaps the pane along the border. When several do, the topmost or
// leftmost wins, as in tmux when no pane was focused before.
//
// Neighbor uses the unzoomed layout, so focus can move while a pane is
// zoomed. ErrNoNeighbor means the pane is at the window edge on that side.
func Neighbor(t *Tree, id PaneID, d Direction, cols, rows int) (PaneID, error) {
	if !d.valid() {
		return 0, fmt.Errorf("%w: %d", ErrInvalidDirection, int(d))
	}
	if _, _, ok := t.find(id); !ok {
		return 0, fmt.Errorf("%w: %d", ErrNotFound, id)
	}
	g := place(t.Root, Rect{W: max(cols, 0), H: max(rows, 0)})
	from := g.Panes[id]
	var best PaneID
	var bestPos int
	for _, c := range t.Panes() {
		r := g.Panes[c]
		var adjacent bool
		var pos int
		switch d {
		case Left:
			adjacent, pos = r.X+r.W+1 == from.X && overlap(r.Y, r.H, from.Y, from.H), r.Y
		case Right:
			adjacent, pos = from.X+from.W+1 == r.X && overlap(r.Y, r.H, from.Y, from.H), r.Y
		case Up:
			adjacent, pos = r.Y+r.H+1 == from.Y && overlap(r.X, r.W, from.X, from.W), r.X
		case Down:
			adjacent, pos = from.Y+from.H+1 == r.Y && overlap(r.X, r.W, from.X, from.W), r.X
		}
		if adjacent && (best == 0 || pos < bestPos) {
			best, bestPos = c, pos
		}
	}
	if best == 0 {
		return 0, fmt.Errorf("%w: pane %d", ErrNoNeighbor, id)
	}
	return best, nil
}

// overlap reports whether spans [a, a+alen) and [b, b+blen) share a cell.
func overlap(a, alen, b, blen int) bool {
	return a < b+blen && b < a+alen
}
