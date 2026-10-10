package layout

import (
	"fmt"
	"math"
)

// ratioTolerance absorbs floating-point error in a sum of ratios.
const ratioTolerance = 1e-9

// validate reports the first broken invariant of t, wrapped in
// ErrInvalidTree.
func (t *Tree) validate() error {
	if t.Root == nil {
		return fmt.Errorf("%w: no root", ErrInvalidTree)
	}
	seen := map[PaneID]bool{}
	if err := validateNode(t.Root, seen); err != nil {
		return err
	}
	if t.Zoomed != 0 && !seen[t.Zoomed] {
		return fmt.Errorf("%w: zoomed pane %d is not in the tree", ErrInvalidTree, t.Zoomed)
	}
	return nil
}

func validateNode(n *Node, seen map[PaneID]bool) error {
	if n == nil {
		return fmt.Errorf("%w: nil node", ErrInvalidTree)
	}
	if n.leaf() {
		switch {
		case n.Pane <= 0:
			return fmt.Errorf("%w: leaf pane ID %d", ErrInvalidTree, n.Pane)
		case n.Axis != 0 || n.Ratios != nil:
			return fmt.Errorf("%w: leaf %d has an axis or ratios", ErrInvalidTree, n.Pane)
		case seen[n.Pane]:
			return fmt.Errorf("%w: pane %d appears twice", ErrInvalidTree, n.Pane)
		}
		seen[n.Pane] = true
		return nil
	}
	switch {
	case n.Pane != 0:
		return fmt.Errorf("%w: split has pane ID %d", ErrInvalidTree, n.Pane)
	case n.Axis != Horizontal && n.Axis != Vertical:
		return fmt.Errorf("%w: split axis %d", ErrInvalidTree, int(n.Axis))
	case len(n.Children) < 2:
		return fmt.Errorf("%w: split has %d child", ErrInvalidTree, len(n.Children))
	case len(n.Ratios) != len(n.Children):
		return fmt.Errorf("%w: %d ratios for %d children", ErrInvalidTree, len(n.Ratios), len(n.Children))
	}
	var sum float64
	for _, r := range n.Ratios {
		if !(r > 0) || math.IsInf(r, 0) {
			return fmt.Errorf("%w: ratio %v", ErrInvalidTree, r)
		}
		sum += r
	}
	if math.Abs(sum-1) > ratioTolerance {
		return fmt.Errorf("%w: ratios sum to %v", ErrInvalidTree, sum)
	}
	for _, c := range n.Children {
		if err := validateNode(c, seen); err != nil {
			return err
		}
	}
	return nil
}
