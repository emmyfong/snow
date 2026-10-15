package layout

import "fmt"

// MinRatio is the smallest share a resize leaves to a child of a split. It
// keeps every ratio positive. 1/16 is exact in binary floating point, so a
// clamped pair still sums to its old total without rounding error.
const MinRatio = 1.0 / 16

// Resize moves the edge of pane id on side d by delta, a share of the split
// that owns that edge. A positive delta moves the edge outward and grows the
// pane; a negative delta shrinks it. The sibling across the edge gives or
// takes the same share. Both are clamped to at least MinRatio.
//
// The edge belongs to the nearest split on d's axis where the pane's branch
// has a sibling on side d. ErrNoEdge means there is none, for example at the
// window border. Resize unzooms the tree, as tmux does.
func (t *Tree) Resize(id PaneID, d Direction, delta float64) error {
	if !d.valid() {
		return fmt.Errorf("%w: %d", ErrInvalidDirection, int(d))
	}
	_, path, ok := t.find(id)
	if !ok {
		return fmt.Errorf("%w: %d", ErrNotFound, id)
	}
	s, j, ok := edgeOwner(path, d)
	if !ok {
		return fmt.Errorf("%w: pane %d", ErrNoEdge, id)
	}
	r := s.split.Ratios
	sum := r[s.index] + r[j]
	grown := min(max(r[s.index]+delta, MinRatio), sum-MinRatio)
	if sum < 2*MinRatio {
		grown = r[s.index]
	}
	r[s.index], r[j] = grown, sum-grown
	t.Zoomed = 0
	return nil
}

// ResizeCells is Resize with the move given in cells, for a window of cols by
// rows. The cells are measured in the split that owns the edge, so a nested
// pane moves by the same number of cells as a top-level one.
func (t *Tree) ResizeCells(id PaneID, d Direction, cells, cols, rows int) error {
	if !d.valid() {
		return fmt.Errorf("%w: %d", ErrInvalidDirection, int(d))
	}
	_, path, ok := t.find(id)
	if !ok {
		return fmt.Errorf("%w: %d", ErrNotFound, id)
	}
	s, _, ok := edgeOwner(path, d)
	if !ok {
		return fmt.Errorf("%w: pane %d", ErrNoEdge, id)
	}
	r, _ := rectOf(t.Root, s.split, Rect{W: max(cols, 0), H: max(rows, 0)})
	length := r.H
	if s.split.Axis == Horizontal {
		length = r.W
	}
	avail := length - (len(s.split.Children) - 1)
	if avail <= 0 {
		return nil
	}
	return t.Resize(id, d, float64(cells)/float64(avail))
}

// edgeOwner returns the step on path whose split owns the pane's edge on
// side d, and the index of the sibling across that edge: the nearest split
// on d's axis where the pane's branch has a sibling on side d.
func edgeOwner(path []step, d Direction) (step, int, bool) {
	for k := len(path) - 1; k >= 0; k-- {
		s := path[k]
		if s.split.Axis != d.axis() {
			continue
		}
		j := s.index - 1
		if d.forward() {
			j = s.index + 1
		}
		if j >= 0 && j < len(s.split.Children) {
			return s, j, true
		}
	}
	return step{}, 0, false
}
