package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

// PanelOptions configures Panel.
type PanelOptions struct {
	Title string
	Body  string // rows separated by newlines; may hold styles
	Width int    // total width, frame included
}

// Panel draws Body in a frame exactly Width cells wide, with Title in the top
// edge. Body rows that do not fit are cut. It draws nothing narrower than 4
// cells.
func Panel(th theme.Theme, o PanelOptions) string {
	if o.Width < 4 {
		return ""
	}
	var rows []string
	if o.Body != "" {
		rows = strings.Split(o.Body, "\n")
	}
	return frame(th, o.Title, rows, o.Width)
}

// frame draws rows inside a border of total width w, title in the top edge.
// Rows are cut or padded to the inside width.
func frame(th theme.Theme, title string, rows []string, w int) string {
	g := th.Glyphs
	edge := lipgloss.NewStyle().Foreground(th.FocusBorder)
	inner := w - 2
	title = ansi.Truncate(title, max(inner-2, 0), "")
	top := g.TopLeft + g.Horizontal
	if title != "" {
		top += " " + title + " "
	}
	top = ansi.Truncate(top, w-1, "")
	top += strings.Repeat(g.Horizontal, max(w-1-ansi.StringWidth(top), 0)) + g.TopRight
	out := []string{edge.Render(top)}
	for _, r := range rows {
		r = ansi.Truncate(r, inner, "")
		r += strings.Repeat(" ", inner-ansi.StringWidth(r))
		out = append(out, edge.Render(g.Vertical)+r+edge.Render(g.Vertical))
	}
	out = append(out, edge.Render(g.BottomLeft+strings.Repeat(g.Horizontal, inner)+g.BottomRight))
	return strings.Join(out, "\n")
}
