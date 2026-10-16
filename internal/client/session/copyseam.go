package session

// startPaneDrag is the seam for tmux's drag-to-copy (#55). A left press in a
// pane whose program did not ask for the mouse calls it with the pane ID and
// the cell inside the pane. Copy mode is built on another branch, so this
// does nothing yet. When it merges, this should enter copy mode at that cell,
// extend the selection on motion, and copy on release. Copy mode must then
// take mouse events first (see mouseMsg), so the drag never reaches the
// border code.
func (m *Model) startPaneDrag(_, _, _ int) {}
