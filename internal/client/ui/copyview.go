package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

// CopyViewOptions configures CopyView.
type CopyViewOptions struct {
	Width, Height int
	Lines         []string // styled rows; row i is drawn at row i
	Indicator     string   // drawn in the top-right corner, for example "[12/5000]"
}

// CopyView draws a pane's history in copy mode into exactly Width by Height
// cells: Lines cut or padded as PaneView does, and Indicator over the right
// end of the first row. The indicator is cut to fit, and dropped when it
// would cover the whole row.
func CopyView(th theme.Theme, o CopyViewOptions) string {
	if o.Width <= 0 || o.Height <= 0 {
		return ""
	}
	rows := make([]string, o.Height)
	for i := range rows {
		rows[i] = paneRow(o.Lines, i, o.Width)
	}
	if o.Indicator != "" && o.Width > 1 {
		mark := ansi.Truncate(o.Indicator, o.Width-1, "")
		w := ansi.StringWidth(mark)
		style := lipgloss.NewStyle().Foreground(th.IndicatorFG).Background(th.IndicatorBG)
		rows[0] = ansi.Cut(rows[0], 0, o.Width-w) + ansi.ResetStyle + style.Render(mark)
	}
	return strings.Join(rows, "\n")
}
