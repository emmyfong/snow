package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

// ApprovalOptions configures ApprovalBox.
type ApprovalOptions struct {
	Width   int
	Title   string   // the question, for example "Allow write?"
	Body    []string // rows above the choices; may hold styles
	Choices []string // button labels, in order
	// Selected is the index of the highlighted choice.
	Selected int
	// Reason replaces the choices with a field for why the answer is no.
	Reason      bool
	ReasonValue string
}

// approvalHints are the keys a reason field takes.
var approvalHints = []Hint{{Key: "Enter", Desc: "answer no"}, {Key: "Esc", Desc: "back"}}

// ApprovalBox draws a question about a tool call in a frame exactly Width
// cells wide, in the prompt colors: the title in the top edge, Body, then
// the choices as buttons with Selected highlighted. The buttons share a row
// when they fit and take one row each when they do not. With Reason set, a
// text field and its keys replace them. Nothing below 4 cells.
func ApprovalBox(th theme.Theme, o ApprovalOptions) string {
	if o.Width < 4 {
		return ""
	}
	inner := o.Width - 2
	rows := append([]string(nil), o.Body...)
	rows = append(rows, "")
	if o.Reason {
		rows = append(rows,
			Input(th, InputOptions{Width: inner, Prompt: " Reason: ", Value: o.ReasonValue, Placeholder: "optional", Focused: true}),
			" "+KeyHints(th, approvalHints, inner-1))
	} else {
		rows = append(rows, buttonRows(th, o)...)
	}
	title := lipgloss.NewStyle().Foreground(th.PromptFG).Background(th.PromptBG).Render(" " + plainLine(o.Title) + " ")
	return frameIn(th, th.PromptBG, title, rows, o.Width)
}

// ButtonSpan is where one choice of an ApprovalBox sits, in cells from the
// box's top-left corner.
type ButtonSpan struct {
	Index   int
	X, Y, W int
}

// ApprovalButtons returns where each choice sits in the box ApprovalBox
// draws for the same options, so a click maps to a choice. None while the
// reason field shows.
func ApprovalButtons(o ApprovalOptions) []ButtonSpan {
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
		out[i] = " " + plainLine(c) + " "
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

func buttonRows(th theme.Theme, o ApprovalOptions) []string {
	on := lipgloss.NewStyle().Foreground(th.PromptFG).Background(th.PromptBG).Bold(true)
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
