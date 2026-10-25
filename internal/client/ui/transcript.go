package ui

import (
	"image/color"
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

// EntryKind says how Transcript draws an entry.
type EntryKind int

// Kinds of transcript entry.
const (
	EntryUser     EntryKind = iota // a message the user sent, under the "you" label
	EntryAgent                     // the agent's reply, under the "snow" label
	EntryThinking                  // the agent's reasoning, muted
	EntryNote                      // a note on the conversation, muted, such as a model switch
	EntryError                     // a failure, under the "error" label
	EntryQueued                    // a message that waits for the running turn
	EntryModel                     // a reply from another model than asked for, under the "model" label
)

// TranscriptEntry is one entry of a chat transcript, as plain text.
type TranscriptEntry struct {
	Kind EntryKind
	Text string
	// Cut marks a reply that stopped before the agent finished it.
	Cut bool
}

// TranscriptOptions configures Transcript.
type TranscriptOptions struct {
	Width   int
	Entries []TranscriptEntry
}

// labelWidth is the column of the "you" and "snow" labels, and the gap
// after it; text lines up after both.
const (
	labelWidth = 6
	labelGap   = 1
)

// Transcript draws entries as rows at most Width cells wide, with a blank
// row between entries. Messages and replies start with a label; their
// wrapped lines line up after it. Text is drawn as plain text: escape
// sequences and control characters in it are dropped, so a reply cannot
// restyle or move the screen.
func Transcript(th theme.Theme, o TranscriptOptions) string {
	if o.Width <= 0 || len(o.Entries) == 0 {
		return ""
	}
	indent := TranscriptIndent(o.Width)
	muted := lipgloss.NewStyle().Foreground(th.Muted)
	var rows []string
	for i, e := range o.Entries {
		if i > 0 {
			rows = append(rows, "")
		}
		text := plainText(e.Text)
		style := lipgloss.NewStyle()
		label := ""
		switch e.Kind {
		case EntryUser:
			label = badge(" you", th.UserFG, th.UserBG)
		case EntryAgent:
			label = badge(" snow", th.AgentFG, th.AgentBG)
		case EntryError:
			label = badge(" error", th.ErrorFG, th.Error)
		case EntryModel:
			label = badge(" model", th.ErrorFG, th.Error)
		case EntryThinking:
			style = muted.Italic(true)
		case EntryNote:
			style = muted
		case EntryQueued:
			style = lipgloss.NewStyle().Foreground(th.Queued)
		}
		lines := wrapText(text, o.Width-indent)
		if e.Cut {
			lines = append(lines, muted.Render("(cut off)"))
		}
		if label != "" && indent == 0 {
			rows = append(rows, ansi.Truncate(label, o.Width, ""))
			label = ""
		}
		for j, l := range lines {
			lead := strings.Repeat(" ", indent)
			if j == 0 && label != "" {
				lead = label + strings.Repeat(" ", labelGap)
			}
			cutNote := e.Cut && j == len(lines)-1 // already styled
			if l != "" && !cutNote {
				l = style.Render(l)
			}
			// A wide character cannot wrap into a width of one cell.
			rows = append(rows, ansi.Truncate(lead+l, o.Width, ""))
		}
		if len(lines) == 0 && label != "" {
			rows = append(rows, label)
		}
	}
	return strings.Join(rows, "\n")
}

// TranscriptIndent is the column where Transcript starts text at width:
// after the label column, or 0 when too narrow for one, where labels take
// their own row. Rows drawn beside a transcript line up with it.
func TranscriptIndent(width int) int {
	if width < labelWidth+labelGap+8 {
		return 0
	}
	return labelWidth + labelGap
}

// badge draws a label padded to labelWidth in fg on bg.
func badge(text string, fg, bg color.Color) string {
	return lipgloss.NewStyle().Foreground(fg).Background(bg).Render(text + strings.Repeat(" ", max(labelWidth-ansi.StringWidth(text), 0)))
}

// wrapText wraps text to width cells, at spaces where it can and inside
// long words where it must.
func wrapText(text string, width int) []string {
	if text == "" {
		return nil
	}
	if width <= 0 {
		width = 1
	}
	return strings.Split(ansi.Wrap(text, width, ""), "\n")
}

// plainText drops escape sequences and control characters other than line
// breaks, and turns tabs into spaces.
func plainText(s string) string {
	s = ansi.Strip(strings.ReplaceAll(s, "\r\n", "\n"))
	s = strings.ReplaceAll(s, "\t", "    ")
	return strings.Map(func(r rune) rune {
		if r != '\n' && unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}
