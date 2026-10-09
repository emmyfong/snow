package client

import "github.com/emmyfong/snow/pkg/api"

// acceptedSize limits a terminal size to what servers accept. A larger
// terminal shows the pane in its top-left corner.
func acceptedSize(cols, rows int) (int, int) {
	return min(max(cols, 1), api.MaxCols), min(max(rows, 1), api.MaxRows)
}
