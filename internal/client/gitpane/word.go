package gitpane

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

// changeWord maps either side of a change to its status colors. A conflict
// overrides both sides, including conflicts git marks with another code.
func changeWord(th theme.Theme, e entry) (word string, fg, bg color.Color) {
	code := e.change.Work
	if e.staged {
		code = e.change.Index
	}
	if e.change.Conflict || code == "U" {
		return "conflict", th.ErrorFG, th.Error
	}
	switch code {
	case "A":
		return "added", th.Added, nil
	case "M", "T":
		return "modified", th.Modified, nil
	case "D":
		return "deleted", th.Removed, nil
	case "R":
		return "renamed", th.Accent, nil
	case "C":
		return "copied", th.Accent, nil
	case "?":
		return "untracked", th.Muted, nil
	default:
		return "changed", th.Muted, nil
	}
}

// changeCell keeps the ten-cell word column separate from the path, so the
// path's left cut preserves the filename even in a narrow pane.
func (m *Model) changeCell(e entry, width int) string {
	const wordWidth = 10
	word, fg, bg := changeWord(m.th, e)
	style := lipgloss.NewStyle().Foreground(fg)
	selected := m.mode != modeCommit && m.selKey == e.key()
	if selected && bg == nil {
		// Status foregrounds target the normal background. Selection has
		// its own contrast pair; conflicts retain their error chip.
		style = style.Foreground(m.th.SelectionFG)
		bg = m.th.Selection
	}
	if bg != nil {
		style = style.Background(bg)
	}
	rest := strings.Repeat(" ", wordWidth-len(word)) + cutLeft(pathText(e.change), max(width-wordWidth, 0))
	if selected {
		rest += strings.Repeat(" ", max(width-len(word)-ansi.StringWidth(rest), 0))
		rest = lipgloss.NewStyle().Foreground(m.th.SelectionFG).Background(m.th.Selection).Render(rest)
	}
	return style.Render(word) + rest
}
