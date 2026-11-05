package layout

import "fmt"

// The smallest pane SplitWithin creates, in cells. The values match tmux's
// pane minimum.
const (
	MinCols = 1
	MinRows = 1
)

// SplitWithin is Split for a window of cols by rows cells. It refuses with
// ErrTooSmall when either resulting pane would be smaller than MinCols by
// MinRows. The check uses the unzoomed layout, because Split unzooms.
func (t *Tree) SplitWithin(target PaneID, d Direction, newPane PaneID, cols, rows int) error {
	trial, err := t.trySplitWithin(target, d, newPane, cols, rows)
	if err != nil {
		return err
	}
	*t = *trial
	return nil
}

// CanSplitWithin returns the error SplitWithin would return, and changes
// nothing.
func (t *Tree) CanSplitWithin(target PaneID, d Direction, newPane PaneID, cols, rows int) error {
	_, err := t.trySplitWithin(target, d, newPane, cols, rows)
	return err
}

// trySplitWithin returns a copy of t with SplitWithin applied.
func (t *Tree) trySplitWithin(target PaneID, d Direction, newPane PaneID, cols, rows int) (*Tree, error) {
	trial := t.clone()
	if err := trial.Split(target, d, newPane); err != nil {
		return nil, err
	}
	g := place(trial.Root, Rect{W: max(cols, 0), H: max(rows, 0)})
	for _, id := range []PaneID{target, newPane} {
		if r := g.Panes[id]; r.W < MinCols || r.H < MinRows {
			return nil, fmt.Errorf("%w: pane %d would be %dx%d in a %dx%d window, minimum %dx%d",
				ErrTooSmall, id, r.W, r.H, cols, rows, MinCols, MinRows)
		}
	}
	return trial, nil
}
