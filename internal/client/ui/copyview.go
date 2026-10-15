package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

// HighlightKind says how a Highlight is drawn. A later kind covers an
// earlier one where they overlap.
type HighlightKind int

// Highlight kinds.
const (
	HighlightMatch   HighlightKind = iota + 1 // a search match
	HighlightCurrent                          // the match the cursor is on
)

// Highlight marks Len cells of a row, from column Col.
type Highlight struct {
	Row, Col, Len int
	Kind          HighlightKind
}

// CopyViewOptions configures CopyView.
type CopyViewOptions struct {
	Width, Height int
	Lines         []string // styled rows; row i is drawn at row i
	Highlights    []Highlight
	Indicator     string // drawn in the top-right corner, for example "[12/5000]"
}

// CopyView draws a pane's history in copy mode into exactly Width by Height
// cells: Lines cut or padded as PaneView does, Highlights painted over them
// in the theme's colors, and Indicator over the right end of the first row.
// A highlight replaces the styles of the text it covers and is cut at the
// row's edge. The indicator is cut to fit, and dropped when it would cover
// the whole row.
func CopyView(th theme.Theme, o CopyViewOptions) string {
	if o.Width <= 0 || o.Height <= 0 {
		return ""
	}
	rows := make([]string, o.Height)
	for i := range rows {
		rows[i] = paneRow(o.Lines, i, o.Width)
	}
	for i := range rows {
		rows[i] = paint(th, rows[i], o.Width, highlightsOf(o.Highlights, i))
	}
	if o.Indicator != "" && o.Width > 1 {
		mark := ansi.Truncate(o.Indicator, o.Width-1, "")
		w := ansi.StringWidth(mark)
		style := lipgloss.NewStyle().Foreground(th.IndicatorFG).Background(th.IndicatorBG)
		rows[0] = ansi.Cut(rows[0], 0, o.Width-w) + ansi.ResetStyle + style.Render(mark)
	}
	return strings.Join(rows, "\n")
}

func highlightsOf(all []Highlight, row int) []Highlight {
	var out []Highlight
	for _, h := range all {
		if h.Row == row && h.Len > 0 && h.Col >= 0 {
			out = append(out, h)
		}
	}
	return out
}

// paint draws hs over row, which is exactly width cells wide. Each cell
// takes the highest kind that covers it, and runs of one kind are drawn
// together.
func paint(th theme.Theme, row string, width int, hs []Highlight) string {
	if len(hs) == 0 {
		return row
	}
	kinds := make([]HighlightKind, width)
	for _, h := range hs {
		for x := h.Col; x < min(h.Col+h.Len, width); x++ {
			kinds[x] = max(kinds[x], h.Kind)
		}
	}
	var b strings.Builder
	for x := 0; x < width; {
		end := x + 1
		for end < width && kinds[end] == kinds[x] {
			end++
		}
		part := ansi.Cut(row, x, end)
		if kinds[x] == 0 {
			b.WriteString(part)
		} else {
			b.WriteString(ansi.ResetStyle)
			b.WriteString(highlightStyle(th, kinds[x]).Render(ansi.Strip(part)))
		}
		x = end
	}
	return b.String()
}

func highlightStyle(th theme.Theme, k HighlightKind) lipgloss.Style {
	var fg, bg color.Color
	switch k {
	case HighlightCurrent:
		fg, bg = th.CurrentFG, th.CurrentBG
	default:
		fg, bg = th.MatchFG, th.MatchBG
	}
	return lipgloss.NewStyle().Foreground(fg).Background(bg)
}
