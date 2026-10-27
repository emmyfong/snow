package ui

import (
	"charm.land/lipgloss/v2"
	"github.com/emmyfong/snow/internal/client/theme"
)

// Mark is the state of the agents in a window, as its status bar tab shows
// it.
type Mark int

// Marks, least urgent first. MarkNone draws nothing.
const (
	MarkNone     Mark = iota
	MarkIdle          // waits for a task
	MarkWorking       // runs a turn
	MarkNeedsYou      // waits for the user
)

// glyph returns the theme glyph for m, or "" for MarkNone.
func (m Mark) glyph(g theme.Glyphs) string {
	switch m {
	case MarkIdle:
		return g.AgentIdle
	case MarkWorking:
		return g.AgentWorking
	case MarkNeedsYou:
		return g.AgentNeedsYou
	}
	return ""
}

// tabStyle is how a tab is drawn: the shown window inverted, a window
// whose agent waits for the user in the prompt colors, so it stands out
// from both, and any other in the bar's colors.
func tabStyle(th theme.Theme, t Tab, bar lipgloss.Style) lipgloss.Style {
	switch {
	case t.Active:
		return lipgloss.NewStyle().Foreground(th.TabActive).Background(th.TabActiveBG).Bold(true)
	case t.Agent == MarkNeedsYou:
		return lipgloss.NewStyle().Foreground(th.PromptFG).Background(th.PromptBG).Bold(true)
	}
	return bar
}
