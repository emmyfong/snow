package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

// StatusOptions configures StatusBar.
type StatusOptions struct {
	Width   int
	Session string
	Folder  string
	Right   string // the needs-you count or copy-mode status
	Message string // shown in place of Right, in the error color
	Notice  string // shown in place of Right, in the bar's color; a Message wins
	Prompt  string // a question for the user; replaces steady status content
	Hint    string // a faint reminder before Right; dropped first when the bar is short
	Mode    string // the state of the pane whose history is shown, kept at the right end
}

// StatusBar draws one line of exactly Width cells: the session name and folder
// on the left, and Hint and Right, or Message, or Notice, on the
// right. When the line is too short, Hint is dropped first, then Right; but a
// Message or a Notice stays and the left part is cut to make room for it; one
// wider than the line is cut too. A Prompt replaces steady status content and
// keeps Hint when both fit. Mode keeps the right end: the left part gives way
// to it first, and it is cut only when it alone is wider than the line.
func StatusBar(th theme.Theme, o StatusOptions) string {
	if o.Width <= 0 {
		return ""
	}
	if o.Prompt != "" {
		// A question takes the whole bar in its own colors, as in tmux, so
		// it cannot be missed. The mode text stays at the right end, and it
		// names the exit key, so the hint gives way to it.
		prompt := lipgloss.NewStyle().Foreground(th.PromptFG).Background(th.PromptBG).Bold(true)
		text := " " + PlainLine(o.Prompt)
		mode := PlainLine(o.Mode)
		limit := o.Width
		if mode != "" {
			if mw := ansi.StringWidth(mode); mw <= o.Width {
				limit = max(o.Width-mw, 0)
			} else {
				mode = ""
			}
		}
		text = ansi.Truncate(text, limit, "")
		hint := ""
		if o.Hint != "" && mode == "" {
			hint = PlainLine(o.Hint) + " "
			if ansi.StringWidth(text)+ansi.StringWidth(hint)+1 > o.Width {
				hint = ""
			}
		}
		gap := o.Width - ansi.StringWidth(text) - ansi.StringWidth(hint) - ansi.StringWidth(mode)
		out := prompt.Render(text + strings.Repeat(" ", gap))
		if hint != "" {
			out += prompt.Faint(true).Render(hint)
		}
		if mode != "" {
			out += prompt.Render(mode)
		}
		return out
	}
	bar := lipgloss.NewStyle().Foreground(th.BarFG).Background(th.BarBG)
	l := layoutStatus(th, o)
	rightStyle := bar
	if o.Message != "" {
		// Its own strip, so the message reads on any status bar color.
		rightStyle = lipgloss.NewStyle().Foreground(th.ErrorFG).Background(th.Error).Bold(true)
	}
	left := ansi.Truncate(l.left, l.leftMax, "")
	hint := ""
	if l.hint != "" {
		// Faint keeps the bar's own color, so the reminder reads on the accent.
		hint = bar.Faint(true).Render(l.hint)
	}
	divider := ""
	if o.Message != "" && l.right != "" {
		// A dark cell, because the error strip can share the bar's color.
		divider = lipgloss.NewStyle().Background(th.BarFG).Render(" ")
	}
	gap := max(o.Width-ansi.StringWidth(left)-ansi.StringWidth(hint)-ansi.StringWidth(divider)-ansi.StringWidth(l.right)-ansi.StringWidth(l.sep)-ansi.StringWidth(l.mode), 0)
	mode := ""
	if l.mode != "" {
		mode = bar.Render(l.mode)
	}
	sep := ""
	if l.sep != "" {
		sep = bar.Render(l.sep)
	}
	return left + bar.Render(strings.Repeat(" ", gap)) + hint + divider + rightStyle.Render(l.right) + sep + mode
}

// statusLayout is the bar's pieces before they are cut to width.
type statusLayout struct {
	left    string
	hint    string
	right   string
	sep     string // the gap between right and mode
	mode    string
	leftMax int
}

func layoutStatus(th theme.Theme, o StatusOptions) statusLayout {
	bar := lipgloss.NewStyle().Foreground(th.BarFG).Background(th.BarBG)
	text := PlainLine(o.Session)
	if o.Folder != "" {
		text += " · " + PlainLine(o.Folder)
	}
	left := bar.Render(" " + text + " ")
	mode := PlainLine(o.Mode)
	width := o.Width
	if mode != "" {
		if mw := ansi.StringWidth(mode); mw < o.Width {
			width = o.Width - mw // the mode text keeps its place at the right
		} else {
			// Mode alone fills the bar, or more: cut it from its end.
			return statusLayout{mode: ansi.Truncate(mode, o.Width, "")}
		}
	}
	right := PlainLine(o.Right)
	shown := PlainLine(o.Message)
	if shown == "" {
		shown = PlainLine(o.Notice)
	}
	if shown != "" {
		right = shown
	}
	right = " " + right + " "
	hint := ""
	if o.Hint != "" && shown == "" {
		hint = PlainLine(o.Hint) + "  "
	}
	sep := ""
	if mode != "" && (shown != "" || PlainLine(o.Right) != "") {
		sep = "  " // so the mode text does not run into the count or the notice
	}
	lw, rw, hw := ansi.StringWidth(left), ansi.StringWidth(right), ansi.StringWidth(hint)
	divider := 0
	if o.Message != "" {
		divider = 1 // the divider StatusBar draws before the message
	}
	rw += divider + ansi.StringWidth(sep)
	leftMax := width
	switch {
	case lw+rw+hw <= width:
	case hw > 0 && lw+rw <= width:
		// The hint gives way to steady status content.
		hint = ""
	case (shown != "" || mode != "") && rw < width:
		// A refusal or a notice is the only feedback the user gets, and the
		// mode text always shows, so the session and folder make room.
		leftMax = width - rw
	case shown != "":
		// Too wide even alone: cut it, and give it the whole bar but the
		// divider and the mode text's gap.
		available := max(width-divider-ansi.StringWidth(sep), 0)
		right = ansi.Truncate(" "+ansi.Truncate(shown, max(available-2, 0), "…")+" ", available, "")
		leftMax = max(width-ansi.StringWidth(right)-divider-ansi.StringWidth(sep), 0)
	default:
		// Too wide for Right too: drop it, and the hint with it.
		right, hint, sep = "", "", ""
	}
	if right == "" {
		// A gap needs a piece on both sides. Without one it only overflows.
		sep = ""
	}
	return statusLayout{left: left, hint: hint, right: right, sep: sep, mode: mode, leftMax: leftMax}
}
