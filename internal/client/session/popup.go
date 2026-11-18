package session

import (
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
)

// popup is a box drawn over the window area: the right-click menu and the
// confirmation both are one. View returns the box and the cell of its
// top-left corner for a window area of cols by rows cells, or "" when it
// does not fit.
type popup interface {
	View(th theme.Theme, cols, rows int) (box string, x, y int)
}

// overlayPopup draws p over content, the window area of rows rows.
func (m *Model) overlayPopup(content string, p popup, rows int) string {
	box, x, y := p.View(m.theme, m.cols, rows)
	if box == "" {
		return content
	}
	return ui.Overlay(content, box, x, y)
}

// windowRows is the height of the pane area between the window bar and status line.
func (m *Model) windowRows() int { return max(m.rows-2, 0) }

// areaTop is the screen row where the pane area starts.
func (m *Model) areaTop() int { return 1 }
