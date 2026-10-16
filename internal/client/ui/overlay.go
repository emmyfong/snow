package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Overlay draws box over base with its top-left corner at column x, row y.
// Rows and columns of box that fall outside base are cut; base keeps its
// size. Both may hold ANSI styles.
func Overlay(base, box string, x, y int) string {
	rows := strings.Split(base, "\n")
	for i, b := range strings.Split(box, "\n") {
		r := y + i
		if r < 0 || r >= len(rows) {
			continue
		}
		row := rows[r]
		width := ansi.StringWidth(row)
		if x >= width {
			continue
		}
		b = ansi.Truncate(b, width-x, "")
		end := x + ansi.StringWidth(b)
		rows[r] = ansi.Truncate(row, x, "") + b + ansi.TruncateLeft(row, end, "")
	}
	return strings.Join(rows, "\n")
}

// Size returns the width of the widest row of box and its number of rows,
// so a popup can be centered or kept on screen. An empty box is 0 by 0.
func Size(box string) (width, height int) {
	if box == "" {
		return 0, 0
	}
	rows := strings.Split(box, "\n")
	for _, r := range rows {
		width = max(width, ansi.StringWidth(r))
	}
	return width, len(rows)
}
