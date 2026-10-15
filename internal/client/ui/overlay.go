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
