package copymode

import (
	"fmt"

	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
)

// View draws the pane's area: the history from the first visible line, and
// the position in the top-right corner. Lines that have not arrived are
// blank for the moment.
func (m *Model) View(th theme.Theme) string {
	if !m.ready {
		return ui.CopyView(th, ui.CopyViewOptions{Width: m.cols, Height: m.rows, Lines: m.o.Screen, Indicator: "[...]"})
	}
	lines := make([]string, m.rows)
	for i := range lines {
		lines[i] = m.lines[m.top+i]
	}
	return ui.CopyView(th, ui.CopyViewOptions{
		Width: m.cols, Height: m.rows, Lines: lines, Highlights: m.highlights(),
		Indicator: fmt.Sprintf("[%d/%d]", m.cur.line+1, m.total),
	})
}

// Cursor returns the cursor's cell in the pane's area, and false while it is
// not known.
func (m *Model) Cursor() (x, y int, ok bool) {
	if !m.ready {
		return 0, 0, false
	}
	return m.cur.col, m.cur.line - m.top, true
}
