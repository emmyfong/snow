package layout

import "fmt"

// Swap exchanges the places of panes a and b. The shape and ratios stay; the
// panes trade cells. Like other layout changes, it turns zoom off.
func (t *Tree) Swap(a, b PaneID) error {
	na, _, ok := t.find(a)
	if !ok {
		return fmt.Errorf("%w: %d", ErrNotFound, a)
	}
	nb, _, ok := t.find(b)
	if !ok {
		return fmt.Errorf("%w: %d", ErrNotFound, b)
	}
	na.Pane, nb.Pane = nb.Pane, na.Pane
	t.Unzoom()
	return nil
}
