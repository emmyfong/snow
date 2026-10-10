package layout

import "errors"

// Layout errors. Callers match them with errors.Is. Every operation that
// returns one of them leaves the tree unchanged.
var (
	ErrNotFound         = errors.New("layout: pane not found")
	ErrDuplicatePane    = errors.New("layout: pane ID already in the tree")
	ErrInvalidPane      = errors.New("layout: pane ID must be positive")
	ErrInvalidDirection = errors.New("layout: invalid direction")
	ErrInvalidAxis      = errors.New("layout: invalid axis")
	ErrLastPane         = errors.New("layout: cannot close the last pane")
	ErrNoEdge           = errors.New("layout: pane has no movable edge on that side")
	ErrInvalidTree      = errors.New("layout: invalid tree")
)
