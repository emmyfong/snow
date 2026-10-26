package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

// ChoiceOptions configures ChoiceBox.
type ChoiceOptions struct {
	Width   int
	Title   string   // for example "Allow write?" or "Rate limited"
	Body    []string // rows above the choices; may hold styles
	Choices []string // button labels, in order; none draws no button row
	// Selected is the index of the highlighted choice; -1 highlights none.
	Selected int
	// Alert draws the box in the error colors instead of the prompt
	// colors, for a failure.
	Alert bool
	// Reason replaces the choices with a field for why the answer is no.
	Reason      bool
	ReasonValue string
	// ReasonPrompt, ReasonPlaceholder, and ReasonHints change what the
	// field says, for a field that takes another answer than a reason.
	// Empty ones keep " Reason: ", "optional", and the reason's keys.
	ReasonPrompt      string
	ReasonPlaceholder string
	ReasonHints       []Hint
}

// reasonHints are the keys a reason field takes.
var reasonHints = []Hint{{Key: "Enter", Desc: "answer no"}, {Key: "Esc", Desc: "back"}}

// ChoiceBox draws a question, or a failure and what to do about it, in a
// frame exactly Width cells wide, in the prompt colors (the error colors
// with Alert): the title in the top edge, Body, then the choices as
// buttons with Selected highlighted. The buttons share a row when they fit
// and take one row each when they do not. With Reason set, a text field
// and its keys replace them. Nothing below 4 cells.
func ChoiceBox(th theme.Theme, o ChoiceOptions) string {
	if o.Width < 4 {
		return ""
	}
	inner := o.Width - 2
	rows := append([]string(nil), o.Body...)
	if o.Reason || len(o.Choices) > 0 {
		rows = append(rows, "")
	}
	if o.Reason {
		prompt, placeholder, hints := o.ReasonPrompt, o.ReasonPlaceholder, o.ReasonHints
		if prompt == "" {
			prompt, placeholder = " Reason: ", "optional"
		}
		if hints == nil {
			hints = reasonHints
		}
		rows = append(rows,
			Input(th, InputOptions{Width: inner, Prompt: prompt, Value: o.ReasonValue, Placeholder: placeholder, Focused: true}),
			" "+KeyHints(th, hints, inner-1))
	} else if len(o.Choices) > 0 {
		rows = append(rows, buttonRows(th, o)...)
	}
	fg, bg := boxColors(th, o)
	title := lipgloss.NewStyle().Foreground(fg).Background(bg).Render(" " + PlainLine(o.Title) + " ")
	return frameIn(th, bg, title, rows, o.Width)
}

// boxColors are the title and selected button colors: the prompt's, or
// the error's for an alert.
func boxColors(th theme.Theme, o ChoiceOptions) (fg, bg color.Color) {
	if o.Alert {
		return th.ErrorFG, th.Error
	}
	return th.PromptFG, th.PromptBG
}

// ButtonSpan is where one choice of a ChoiceBox sits, in cells from the
// box's top-left corner.
type ButtonSpan struct {
	Index   int
	X, Y, W int
}

// ChoiceButtons returns where each choice sits in the box ChoiceBox
// draws for the same options, so a click maps to a choice. None while the
// reason field shows.
func ChoiceButtons(o ChoiceOptions) []ButtonSpan {
	if o.Width < 4 || o.Reason {
		return nil
	}
	inner := o.Width - 2
	y := 1 + len(o.Body) + 1 // the top edge, the body, a blank row
	labels := buttonLabels(o.Choices)
	var out []ButtonSpan
	if oneRow(labels, inner) {
		x := 2 // the left edge and a space
		for i, l := range labels {
			out = append(out, ButtonSpan{Index: i, X: x, Y: y, W: ansi.StringWidth(l)})
			x += ansi.StringWidth(l) + 1
		}
		return out
	}
	for i, l := range labels {
		out = append(out, ButtonSpan{Index: i, X: 2, Y: y + i, W: min(ansi.StringWidth(l), inner-1)})
	}
	return out
}

// buttonLabels pads each choice with a space on each side.
func buttonLabels(choices []string) []string {
	out := make([]string, len(choices))
	for i, c := range choices {
		out[i] = " " + PlainLine(c) + " "
	}
	return out
}

// oneRow reports whether labels fit in one row of width cells, after a
// leading space and with a space between them.
func oneRow(labels []string, width int) bool {
	w := 1
	for _, l := range labels {
		w += ansi.StringWidth(l) + 1
	}
	return w-1 <= width
}

func buttonRows(th theme.Theme, o ChoiceOptions) []string {
	fg, bg := boxColors(th, o)
	on := lipgloss.NewStyle().Foreground(fg).Background(bg).Bold(true)
	off := lipgloss.NewStyle().Foreground(th.Accent)
	labels := buttonLabels(o.Choices)
	styled := make([]string, len(labels))
	for i, l := range labels {
		if i == o.Selected {
			styled[i] = on.Render(l)
		} else {
			styled[i] = off.Render(l)
		}
	}
	if oneRow(labels, o.Width-2) {
		return []string{" " + strings.Join(styled, " ")}
	}
	rows := make([]string, len(styled))
	for i, s := range styled {
		rows[i] = " " + s
	}
	return rows
}
