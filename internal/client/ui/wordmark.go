package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

// WordmarkHeight is the wordmark's height in rows.
const WordmarkHeight = 4

// wordmarkPixels draws "snow" on a grid of square pixels: each cell holds
// two pixels, one above the other, drawn with half blocks.
var wordmarkPixels = [8]string{
	".####.  #####.  .####.  ##....##",
	"##..##  ##..##  ##..##  ##....##",
	"##....  ##..##  ##..##  ##.##.##",
	".####.  ##..##  ##..##  ##.##.##",
	"....##  ##..##  ##..##  ##.##.##",
	"##..##  ##..##  ##..##  ########",
	".####.  ##..##  .####.  .##..##.",
	"................................",
}

// Wordmark draws Snow's name, centered in width cells, in the Wordmark color.
// It draws nothing when width is too narrow for it.
func Wordmark(th theme.Theme, width int) string {
	w := len(wordmarkPixels[0])
	if width < w {
		return ""
	}
	style := lipgloss.NewStyle().Foreground(th.Wordmark).Bold(true)
	pad := strings.Repeat(" ", (width-w)/2)
	rows := make([]string, 0, WordmarkHeight)
	for r := 0; r < len(wordmarkPixels); r += 2 {
		var b strings.Builder
		for c := range w {
			top, bottom := wordmarkPixels[r][c] == '#', wordmarkPixels[r+1][c] == '#'
			switch {
			case top && bottom:
				b.WriteString("█")
			case top:
				b.WriteString("▀")
			case bottom:
				b.WriteString("▄")
			default:
				b.WriteByte(' ')
			}
		}
		row := pad + style.Render(b.String())
		rows = append(rows, row+strings.Repeat(" ", width-ansi.StringWidth(row)))
	}
	return strings.Join(rows, "\n")
}
