package layout

import "fmt"

// Zoom makes pane id fill the window. Zooming another pane moves the zoom to
// it. The tree's shape does not change.
func (t *Tree) Zoom(id PaneID) error {
	if _, _, ok := t.find(id); !ok {
		return fmt.Errorf("%w: %d", ErrNotFound, id)
	}
	t.Zoomed = id
	return nil
}

// Unzoom turns zoom off. It does nothing when no pane is zoomed.
func (t *Tree) Unzoom() {
	t.Zoomed = 0
}
