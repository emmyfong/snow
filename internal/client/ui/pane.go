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
		b.WriteString(paneRow(lines, i, width))
	}
	return b.String()
}

// paneRow returns row i of lines cut or padded to exactly width cells.
func paneRow(lines []string, i, width int) string {
	var row string
	if i >= 0 && i < len(lines) {
		row = ansi.Truncate(lines[i], width, "")
	}
	return row + strings.Repeat(" ", max(width-ansi.StringWidth(row), 0))
}
