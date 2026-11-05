package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

// Hint is one key and what it does.
type Hint struct {
	Key, Desc string
}

// hintGap separates hints in a row.
const hintGap = 3

// KeyHints draws hints on one row at most width cells wide: each key in the
// accent color, its description muted. Hints that do not fit whole are
// dropped from the end.
func KeyHints(th theme.Theme, hints []Hint, width int) string {
	key := lipgloss.NewStyle().Foreground(th.Accent)
	desc := lipgloss.NewStyle().Foreground(th.Muted)
	var b strings.Builder
	for i, sp := range HintSpans(hints, width) {
		if i > 0 {
			b.WriteString(strings.Repeat(" ", hintGap))
		}
		h := hints[sp.Index]
		b.WriteString(key.Render(h.Key) + " " + desc.Render(h.Desc))
	}
	return b.String()
}

// HintSpan is the cells one hint covers in the row KeyHints draws.
type HintSpan struct {
	Index int // in the hints
	X, W  int
}

// HintSpans returns where each hint that fits sits in the row KeyHints
// draws for the same hints and width, so a click can find its hint.
func HintSpans(hints []Hint, width int) []HintSpan {
	var out []HintSpan
	used := 0
	for i, h := range hints {
		w := ansi.StringWidth(h.Key) + 1 + ansi.StringWidth(h.Desc)
		x := used
		if used > 0 {
			x += hintGap
		}
		if x+w > width {
			break
		}
		out = append(out, HintSpan{Index: i, X: x, W: w})
		used = x + w
	}
	return out
}
