package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// PaneView draws a pane's rows into exactly width by height cells. Long rows
// are cut at width, short ones padded, and missing rows left blank, so the
// result always covers what was drawn before. It returns "" for an empty
// size.
func PaneView(lines []string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	var b strings.Builder
	for i := range height {
		if i > 0 {
			b.WriteByte('\n')
		}
		var row string
		if i < len(lines) {
			row = ansi.Truncate(lines[i], width, "")
		}
		b.WriteString(row)
		b.WriteString(strings.Repeat(" ", width-ansi.StringWidth(row)))
	}
	return b.String()
}
