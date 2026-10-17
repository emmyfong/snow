package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

// ListItem is one row of a List: its cells, left to right.
type ListItem struct {
	Cols []string
}

// ListOptions configures List.
type ListOptions struct {
	Items    []ListItem
	Selected int // index of the highlighted row; out of range means none
	Width    int
	Height   int // rows shown; 0 shows them all
}

// listGap separates the columns of a List.
const listGap = 3

// List draws rows exactly Width cells wide, with columns aligned. The
// selected row is marked and highlighted. With a Height, it shows that many
// rows, scrolled so the selection is visible.
func List(th theme.Theme, o ListOptions) string {
	if o.Width <= 0 || len(o.Items) == 0 {
		return ""
	}
	g := th.Glyphs
	var widths []int
	for _, it := range o.Items {
		for i, c := range it.Cols {
			if i >= len(widths) {
				widths = append(widths, 0)
			}
			widths[i] = max(widths[i], ansi.StringWidth(c))
		}
	}
	first, last := 0, len(o.Items)
	if o.Height > 0 && o.Height < len(o.Items) {
		first = min(max(o.Selected-o.Height+1, 0), len(o.Items)-o.Height)
		last = first + o.Height
	}
	selected := lipgloss.NewStyle().Foreground(th.SelectionFG).Background(th.Selection).Bold(true)
	blank := strings.Repeat(" ", ansi.StringWidth(g.Pointer))
	rows := make([]string, 0, last-first)
	for i := first; i < last; i++ {
		row := blank
		if i == o.Selected {
			row = g.Pointer
		}
		for j, c := range o.Items[i].Cols {
			if j > 0 {
				row += strings.Repeat(" ", listGap)
			}
			row += c
			if j < len(o.Items[i].Cols)-1 {
				row += strings.Repeat(" ", widths[j]-ansi.StringWidth(c))
			}
		}
		row = ansi.Truncate(row, o.Width, "")
		row += strings.Repeat(" ", o.Width-ansi.StringWidth(row))
		if i == o.Selected {
			row = selected.Render(row)
		}
		rows = append(rows, row)
	}
	return strings.Join(rows, "\n")
}
