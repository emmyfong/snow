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
	Prompt  string // a question for the user; takes the whole bar
	Hint    string // a faint reminder before Right; dropped first when the bar is short
}

// StatusBar draws one line of exactly Width cells: the session name and folder
// on the left, and Hint and Right, or Message, or Notice, on the
// right. When the line is too short, Hint is dropped first, then Right; but a
// Message or a Notice stays and the left part is cut to make room for it; one
// wider than the line is cut too. A Prompt replaces the whole bar.
func StatusBar(th theme.Theme, o StatusOptions) string {
	if o.Width <= 0 {
		return ""
	}
	if o.Prompt != "" {
		// A question takes the whole bar in its own colors, as in tmux, so
		// it cannot be missed.
		prompt := lipgloss.NewStyle().Foreground(th.PromptFG).Background(th.PromptBG).Bold(true)
		text := ansi.Truncate(" "+PlainLine(o.Prompt), o.Width, "")
		return prompt.Render(text + strings.Repeat(" ", o.Width-ansi.StringWidth(text)))
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
	gap := max(o.Width-ansi.StringWidth(left)-ansi.StringWidth(hint)-ansi.StringWidth(divider)-ansi.StringWidth(l.right), 0)
	return left + bar.Render(strings.Repeat(" ", gap)) + hint + divider + rightStyle.Render(l.right)
}

// statusLayout is the bar's pieces before they are cut to width.
type statusLayout struct {
	left    string
	hint    string
	right   string
	leftMax int
}

func layoutStatus(th theme.Theme, o StatusOptions) statusLayout {
	bar := lipgloss.NewStyle().Foreground(th.BarFG).Background(th.BarBG)
	text := PlainLine(o.Session)
	if o.Folder != "" {
		text += " · " + PlainLine(o.Folder)
	}
	left := bar.Render(" " + text + " ")
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
	lw, rw, hw := ansi.StringWidth(left), ansi.StringWidth(right), ansi.StringWidth(hint)
	divider := 0
	if o.Message != "" {
		divider = 1 // the divider StatusBar draws before the message
	}
	rw += divider
	leftMax := o.Width
	switch {
	case lw+rw+hw <= o.Width:
	case hw > 0 && lw+rw <= o.Width:
		// The hint gives way to steady status content.
		hint = ""
	case shown != "" && rw < o.Width:
		// A refusal or a notice is the only feedback the user gets, so the
		// session and folder make room for it.
		leftMax = o.Width - rw
	case shown != "":
		// Too wide even alone: cut it, and give it the whole bar but the
		// divider.
		available := max(o.Width-divider, 0)
		right = ansi.Truncate(" "+ansi.Truncate(shown, max(available-2, 0), "…")+" ", available, "")
		leftMax = max(o.Width-ansi.StringWidth(right)-divider, 0)
	default:
		// Too wide for Right too: drop it, and the hint with it.
		right, hint = "", ""
	}
	return statusLayout{left: left, hint: hint, right: right, leftMax: leftMax}
}
