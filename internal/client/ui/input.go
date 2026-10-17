package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

// InputOptions configures Input.
type InputOptions struct {
	Value       string
	Placeholder string // shown, muted, while Value is empty
	Prompt      string // drawn before the value, in the accent color
	Width       int
	Focused     bool // draws a cursor after the value
}

// Input draws a one-line text field exactly Width cells wide. A value too long
// to fit shows its end, where the user types.
func Input(th theme.Theme, o InputOptions) string {
	if o.Width <= 0 {
		return ""
	}
	prompt := ansi.Truncate(o.Prompt, o.Width, "")
	room := o.Width - ansi.StringWidth(prompt)
	cursor := ""
	if o.Focused && room > 0 {
		cursor = " "
		room--
	}
	var text string
	if o.Value == "" {
		text = lipgloss.NewStyle().Foreground(th.Muted).Render(ansi.Truncate(o.Placeholder, room, ""))
		if cursor != "" {
			// The cursor sits before the placeholder, where typing starts.
			text = lipgloss.NewStyle().Reverse(true).Render(cursor) + text
			cursor = ""
		}
	} else {
		text = ansi.TruncateLeft(o.Value, max(ansi.StringWidth(o.Value)-room, 0), "")
	}
	if cursor != "" {
		text += lipgloss.NewStyle().Reverse(true).Render(cursor)
	}
	out := lipgloss.NewStyle().Foreground(th.Accent).Render(prompt) + text
	return out + strings.Repeat(" ", max(o.Width-ansi.StringWidth(out), 0))
}
