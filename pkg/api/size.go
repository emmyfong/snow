package api

import (
	"errors"
	"fmt"
)

// The largest terminal size a server accepts. A screen buffer grows with
// cols*rows, so an unbounded size lets one message exhaust the server's
// memory.
const (
	MaxCols = 1000
	MaxRows = 1000
)

// ErrBadSize is returned by Hello.Validate and Resize.Validate.
var ErrBadSize = errors.New("api: bad size")

// Validate checks that h reports a terminal size servers accept.
func (h *Hello) Validate() error { return validSize(h.Cols, h.Rows) }

// Validate checks that r reports a terminal size servers accept.
func (r *Resize) Validate() error { return validSize(r.Cols, r.Rows) }

func validSize(cols, rows int) error {
	if cols < 1 || cols > MaxCols || rows < 1 || rows > MaxRows {
		return fmt.Errorf("%w: %dx%d is outside 1x1 to %dx%d", ErrBadSize, cols, rows, MaxCols, MaxRows)
	}
	return nil
}
