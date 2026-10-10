package layout

import "fmt"

// Split divides pane target in half and places pane newPane on side d of it.
// The target becomes a two-child split, even inside a split on the same
// axis, so the new pane takes space only from the target. Split unzooms the
// tree, as tmux does, so the new pane is visible.
func (t *Tree) Split(target PaneID, d Direction, newPane PaneID) error {
	if !d.valid() {
		return fmt.Errorf("%w: %d", ErrInvalidDirection, int(d))
	}
	if newPane <= 0 {
		return fmt.Errorf("%w: %d", ErrInvalidPane, newPane)
	}
	n, _, ok := t.find(target)
	if !ok {
		return fmt.Errorf("%w: %d", ErrNotFound, target)
	}
	if _, _, dup := t.find(newPane); dup {
		return fmt.Errorf("%w: %d", ErrDuplicatePane, newPane)
	}
	children := []*Node{{Pane: target}, {Pane: newPane}}
	if !d.forward() {
		children[0], children[1] = children[1], children[0]
	}
	// Turning the leaf into a split in place keeps the parent's pointer and
	// ratio valid, so no parent bookkeeping is needed.
	*n = Node{Axis: d.axis(), Ratios: []float64{0.5, 0.5}, Children: children}
	t.Zoomed = 0
	return nil
}
