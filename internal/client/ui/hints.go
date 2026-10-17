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
	used := 0
	for _, h := range hints {
		w := ansi.StringWidth(h.Key) + 1 + ansi.StringWidth(h.Desc)
		if used > 0 {
			w += hintGap
		}
		if used+w > width {
			break
		}
		if used > 0 {
			b.WriteString(strings.Repeat(" ", hintGap))
		}
		b.WriteString(key.Render(h.Key) + " " + desc.Render(h.Desc))
		used += w
	}
	return b.String()
}
