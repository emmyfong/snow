package layout

import "fmt"

// Even gives every pane in the row or column that holds pane id an equal
// share. Splits nest, so one visible row can be a chain of splits on the same
// axis, for example h(1, h(2, 3)). Even treats that chain as one row: each
// split's ratios follow how many cells of the row each child holds. A child
// on the other axis counts as one cell. A lone pane has nothing to even.
func (t *Tree) Even(id PaneID) error {
	_, path, ok := t.find(id)
	if !ok {
		return fmt.Errorf("%w: %d", ErrNotFound, id)
	}
	if len(path) == 0 {
		return nil
	}
	axis := path[len(path)-1].split.Axis
	top := len(path) - 1
	for top > 0 && path[top-1].split.Axis == axis {
		top--
	}
	even(path[top].split, axis)
	t.Unzoom()
	return nil
}

// even sets the ratios of n and of its descendants on axis so that each cell
// of the row gets the same share. It returns how many cells n holds.
func even(n *Node, axis Axis) int {
	if n.leaf() || n.Axis != axis {
		return 1
	}
	cells := make([]int, len(n.Children))
	total := 0
	for i, c := range n.Children {
		cells[i] = even(c, axis)
		total += cells[i]
	}
	for i := range n.Ratios {
		n.Ratios[i] = float64(cells[i]) / float64(total)
	}
	return total
}
