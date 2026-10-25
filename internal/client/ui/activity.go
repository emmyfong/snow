package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

// ActivityOptions configures Activity.
type ActivityOptions struct {
	Width int
	Frame int    // which spinner frame to show; it counts up
	Text  string // what runs, for example "working 12s"
	Hint  string // drawn at the right end when it fits, for example "Esc interrupt"
}

// Activity draws the one row that says an agent works: a spinner frame
// from the theme, Text, and Hint at the right end, exactly Width cells
// wide in ActivityFG on ActivityBG. Hint is dropped first when short.
func Activity(th theme.Theme, o ActivityOptions) string {
	if o.Width <= 0 {
		return ""
	}
	frames := []rune(th.Glyphs.Spinner)
	left := " "
	if len(frames) > 0 {
		left += string(frames[((o.Frame%len(frames))+len(frames))%len(frames)]) + " "
	}
	left += o.Text
	right := ""
	if o.Hint != "" {
		right = o.Hint + " "
	}
	gap := o.Width - ansi.StringWidth(left) - ansi.StringWidth(right)
	if gap < 1 {
		right, gap = "", o.Width-ansi.StringWidth(left)
	}
	row := ansi.Truncate(left+strings.Repeat(" ", max(gap, 0))+right, o.Width, "")
	row += strings.Repeat(" ", max(o.Width-ansi.StringWidth(row), 0))
	return lipgloss.NewStyle().Foreground(th.ActivityFG).Background(th.ActivityBG).Render(row)
}
