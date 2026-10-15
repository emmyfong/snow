package client

import "github.com/emmyfong/snow/pkg/api"

// acceptedSize limits a terminal size to what servers accept. A larger
// terminal shows the pane in its top-left corner. A size below 1x1 means
// the size is unknown; ok is false, and the caller sends nothing.
func acceptedSize(cols, rows int) (c, r int, ok bool) {
	if cols < 1 || rows < 1 {
		return 0, 0, false
	}
	return min(cols, api.MaxCols), min(rows, api.MaxRows), true
}
