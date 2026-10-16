package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

// WordmarkHeight is the wordmark's height in rows.
const WordmarkHeight = 4

// WordmarkWidth is how many columns the wordmark takes; Wordmark draws
// nothing in fewer.
const WordmarkWidth = 32

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

// Wordmark draws Snow's name, centered in width cells, bold in the terminal's
// own text color, so it reads on a light or a dark background. It takes a
// theme like every component, though no token colors it.
// It draws nothing when width is too narrow for it.
func Wordmark(_ theme.Theme, width int) string {
	w := WordmarkWidth
	if width < w {
		return ""
	}
	style := lipgloss.NewStyle().Bold(true)
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
