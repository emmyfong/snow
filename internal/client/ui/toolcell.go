package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

// ToolState is how far a tool call got.
type ToolState int

// States of a tool call.
const (
	ToolRunning ToolState = iota
	ToolWaiting           // for the user's answer
	ToolDone
	ToolFailed
)

// ToolCellOptions configures ToolCell.
type ToolCellOptions struct {
	Width    int
	Name     string // the tool, for example "read"
	Subject  string // what it works on, for example a path or a command
	Summary  string // what came of it, for example "212 lines"
	Added    int    // lines the call adds; shown with Removed when either is set
	Removed  int
	State    ToolState
	Selected bool // the cell has the keyboard
	Expanded bool
	// Details, shown when Expanded, are rows the caller drew: the input,
	// the diff, the result.
	Details []string
}

// ToolCell draws one tool call at most Width cells wide: a header row with
// the Collapsed or Expanded glyph, the tool's name, its subject, and its
// summary, then, when Expanded, its details indented under it. A failed
// call's header is in Removed. A selected header is in the selection
// colors across the whole width.
func ToolCell(th theme.Theme, o ToolCellOptions) string {
	if o.Width <= 0 {
		return ""
	}
	glyph := th.Glyphs.Collapsed
	if o.Expanded {
		glyph = th.Glyphs.Expanded
	}
	head := glyph + " " + plainLine(o.Name)
	if o.Subject != "" {
		head += " " + plainLine(o.Subject)
	}
	summary := plainLine(o.Summary)
	switch o.State {
	case ToolRunning:
		summary = "running"
	case ToolWaiting:
		summary = "waiting for you"
	case ToolFailed:
		summary = strings.TrimSpace("failed " + summary)
	}
	counts := ""
	if o.Added > 0 || o.Removed > 0 {
		counts = fmt.Sprintf("+%d -%d", o.Added, o.Removed)
	}
	var row string
	if o.Selected {
		text := head
		for _, s := range []string{counts, summary} {
			if s != "" {
				text += "  " + s
			}
		}
		text = ansi.Truncate(text, o.Width, "")
		text += strings.Repeat(" ", o.Width-ansi.StringWidth(text))
		row = lipgloss.NewStyle().Foreground(th.SelectionFG).Background(th.Selection).Render(text)
	} else {
		name := lipgloss.NewStyle().Bold(true)
		if o.State == ToolFailed {
			name = name.Foreground(th.Removed)
		}
		muted := lipgloss.NewStyle().Foreground(th.Muted)
		row = name.Render(head)
		if counts != "" {
			row += "  " + lipgloss.NewStyle().Foreground(th.Added).Render(fmt.Sprintf("+%d", o.Added)) + " " +
				lipgloss.NewStyle().Foreground(th.Removed).Render(fmt.Sprintf("-%d", o.Removed))
		}
		if summary != "" {
			row += "  " + muted.Render(summary)
		}
		row = ansi.Truncate(row, o.Width, "")
	}
	rows := []string{row}
	if o.Expanded {
		for _, d := range o.Details {
			rows = append(rows, ansi.Truncate("  "+d, o.Width, ""))
		}
	}
	return strings.Join(rows, "\n")
}
