package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

// ListItem is one row of a List: its cells, left to right, or a heading
// that starts a group.
type ListItem struct {
	Cols []string
	// Heading draws the row as a muted section title, with no columns and
	// no selection mark. It comes before the first item of its group.
	Heading string
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
	view, _ := ListRows(th, o)
	return view
}

// ListRows draws what List draws and also returns the index in o.Items of
// the item on each row, so a click can find the row it landed on. A heading
// is an item too; its index points at the heading.
func ListRows(th theme.Theme, o ListOptions) (view string, rows []int) {
	if o.Width <= 0 || len(o.Items) == 0 {
		return "", nil
	}
	g := th.Glyphs
	var widths []int
	for _, it := range o.Items {
		if it.Heading != "" {
			continue
		}
		for i, c := range it.Cols {
			if i >= len(widths) {
				widths = append(widths, 0)
			}
			widths[i] = max(widths[i], ansi.StringWidth(c))
		}
	}
	first, last := ListWindow(len(o.Items), o.Selected, o.Height)
	selected := lipgloss.NewStyle().Foreground(th.SelectionFG).Background(th.Selection).Bold(true)
	muted := lipgloss.NewStyle().Foreground(th.Muted)
	blank := strings.Repeat(" ", ansi.StringWidth(g.Pointer))
	lines := make([]string, 0, last-first)
	rows = make([]int, 0, last-first)
	for i := first; i < last; i++ {
		it := o.Items[i]
		if it.Heading != "" {
			row := ansi.Truncate(blank+muted.Render(it.Heading), o.Width, "")
			row += strings.Repeat(" ", o.Width-ansi.StringWidth(row))
			lines = append(lines, row)
			rows = append(rows, i)
			continue
		}
		row := blank
		if i == o.Selected {
			row = g.Pointer
		}
		for j, c := range it.Cols {
			if j > 0 {
				row += strings.Repeat(" ", listGap)
			}
			row += c
			if j < len(it.Cols)-1 {
				row += strings.Repeat(" ", widths[j]-ansi.StringWidth(c))
			}
		}
		row = ansi.Truncate(row, o.Width, "")
		row += strings.Repeat(" ", o.Width-ansi.StringWidth(row))
		if i == o.Selected {
			row = selected.Render(row)
		}
		lines = append(lines, row)
		rows = append(rows, i)
	}
	return strings.Join(lines, "\n"), rows
}

// ListWindow returns the first and last item indexes List shows for a list
// of n items with selection selected and room for height rows. It is shared
// with a screen that maps its clicks to rows.
func ListWindow(n, selected, height int) (first, last int) {
	first, last = 0, n
	if height > 0 && height < n {
		first = min(max(selected-height+1, 0), n-height)
		last = first + height
	}
	return first, last
}
