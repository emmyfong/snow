package layout

import (
	"fmt"
	"slices"
)

// Close removes pane id. Its share goes to the sibling before it, or to the
// sibling after it when it was first, so the other panes keep their size. A
// split left with one child is replaced by that child. Close unzooms the
// tree. The last pane cannot be closed: the caller closes the window instead.
func (t *Tree) Close(id PaneID) error {
	_, path, ok := t.find(id)
	if !ok {
		return fmt.Errorf("%w: %d", ErrNotFound, id)
	}
	if len(path) == 0 {
		return fmt.Errorf("%w: %d", ErrLastPane, id)
	}
	last := path[len(path)-1]
	p, i := last.split, last.index
	share := p.Ratios[i]
	p.Children = slices.Delete(p.Children, i, i+1)
	p.Ratios = slices.Delete(p.Ratios, i, i+1)
	p.Ratios[max(i-1, 0)] += share
	if len(p.Children) == 1 {
		// The parent keeps this node's ratio, so overwriting the node with
		// its only child moves that child up without touching the parent.
		*p = *p.Children[0]
	}
	t.Zoomed = 0
	return nil
}
