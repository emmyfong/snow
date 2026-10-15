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
	trial := t.clone()
	if err := trial.Split(target, d, newPane); err != nil {
		return err
	}
	g := place(trial.Root, Rect{W: max(cols, 0), H: max(rows, 0)})
	for _, id := range []PaneID{target, newPane} {
		if r := g.Panes[id]; r.W < MinCols || r.H < MinRows {
			return fmt.Errorf("%w: pane %d would be %dx%d in a %dx%d window, minimum %dx%d",
				ErrTooSmall, id, r.W, r.H, cols, rows, MinCols, MinRows)
		}
	}
	*t = *trial
	return nil
}
