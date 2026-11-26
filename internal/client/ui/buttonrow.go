package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

// ButtonRowOptions configures ButtonRow.
type ButtonRowOptions struct {
	Width   int      // the row's width in cells
	Choices []string // button labels, left to right
	Right   string   // a label pinned to the right edge, or ""
}

// ButtonRow draws one row of buttons, Width cells wide, in Accent with no
// background. Choices drop from their right end until the row fits. Right
// drops only when it does not fit alone. The row never wraps.
func ButtonRow(th theme.Theme, o ButtonRowOptions) string {
	if o.Width <= 0 {
		return ""
	}
	labels := buttonLabels(o.Choices)
	labels = append(labels, " "+PlainLine(o.Right)+" ")
	style := lipgloss.NewStyle().Foreground(th.Accent)
	var row strings.Builder
	x := 0
	for _, sp := range ButtonRowSpans(o) {
		row.WriteString(strings.Repeat(" ", sp.X-x))
		row.WriteString(style.Render(labels[sp.Index]))
		x = sp.X + sp.W
	}
	row.WriteString(strings.Repeat(" ", o.Width-x))
	return row.String()
}

// ButtonRowSpans returns where each button sits in the row ButtonRow draws
// for the same options, so a click maps to a button. Choice i has Index i.
// Right has Index len(Choices). A dropped button has no span.
func ButtonRowSpans(o ButtonRowOptions) []ButtonSpan {
	if o.Width <= 0 {
		return nil
	}
	rightW := 0
	if o.Right != "" {
		rightW = ansi.StringWidth(PlainLine(o.Right)) + 2
	}
	if rightW > o.Width {
		rightW = 0
	}
	limit := o.Width
	if rightW > 0 {
		limit -= rightW + 1
	}
	var out []ButtonSpan
	x := 0
	for i, l := range buttonLabels(o.Choices) {
		w := ansi.StringWidth(l)
		if x+w > limit {
			break
		}
		out = append(out, ButtonSpan{Index: i, X: x, W: w})
		x += w + 1
	}
	if rightW > 0 {
		out = append(out, ButtonSpan{Index: len(o.Choices), X: o.Width - rightW, W: rightW})
	}
	return out
}

// buttonLabels pads each choice with a space on each side.
func buttonLabels(choices []string) []string {
	out := make([]string, len(choices))
	for i, c := range choices {
		out[i] = " " + PlainLine(c) + " "
	}
	return out
}
