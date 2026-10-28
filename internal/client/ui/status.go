package ui

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

// Tab is one window in the status bar.
type Tab struct {
	Index  int
	Name   string
	Active bool
	Zoomed bool
	Agent  Mark // the most urgent state of the window's agents
	Driven bool // an agent types into one of its panes
}

// StatusOptions configures StatusBar.
type StatusOptions struct {
	Width   int
	Session string
	Tabs    []Tab
	Right   string // for example the pane's profile and the time
	Message string // shown in place of Right, in the error color
	Prompt  string // a question for the user; takes the whole bar
}

// StatusBar draws one line of exactly Width cells: the session name and the
// window tabs on the left, and Right, or Message, on the right. When the
// line is too short, Right is dropped first, but a Message stays and the tabs
// are cut to make room for it; a Message wider than the line is cut too. A
// Prompt replaces the whole bar.
func StatusBar(th theme.Theme, o StatusOptions) string {
	if o.Width <= 0 {
		return ""
	}
	if o.Prompt != "" {
		// A question takes the whole bar in its own colors, as in tmux, so
		// it cannot be missed.
		prompt := lipgloss.NewStyle().Foreground(th.PromptFG).Background(th.PromptBG).Bold(true)
		text := ansi.Truncate(" "+o.Prompt, o.Width, "")
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
	divider := ""
	if o.Message != "" && l.right != "" {
		// A dark cell, because the error strip can share the bar's color.
		divider = lipgloss.NewStyle().Background(th.BarFG).Render(" ")
	}
	gap := max(o.Width-ansi.StringWidth(left)-ansi.StringWidth(divider)-ansi.StringWidth(l.right), 0)
	return left + bar.Render(strings.Repeat(" ", gap)) + divider + rightStyle.Render(l.right)
}

// TabSpan is the columns a window's tab covers in the status bar.
type TabSpan struct {
	Index int // the window index
	X, W  int // first column and width in cells
}

// TabSpans returns where each visible tab sits in the bar StatusBar draws
// for o, so a click can be matched to a window. A tab cut off by the right
// side is clipped to the visible part, and a tab hidden entirely has no
// span. A Prompt takes the whole bar, so it leaves no spans.
func TabSpans(th theme.Theme, o StatusOptions) []TabSpan {
	if o.Width <= 0 || o.Prompt != "" {
		return nil
	}
	l := layoutStatus(th, o)
	var out []TabSpan
	for _, sp := range l.spans {
		if w := min(sp.X+sp.W, l.leftMax) - sp.X; w > 0 {
			out = append(out, TabSpan{Index: sp.Index, X: sp.X, W: w})
		}
	}
	return out
}

// statusLayout is the bar's pieces before they are cut to the width.
type statusLayout struct {
	left    string    // the session and tabs, untruncated
	spans   []TabSpan // each tab's columns inside left
	right   string    // with its padding; "" when dropped
	leftMax int       // how much of left fits
}

func layoutStatus(th theme.Theme, o StatusOptions) statusLayout {
	bar := lipgloss.NewStyle().Foreground(th.BarFG).Background(th.BarBG)
	g := th.Glyphs
	first := bar.Render(" " + g.SessionLeft + o.Session + g.SessionRight + " ")
	parts := []string{first}
	x := ansi.StringWidth(first)
	var spans []TabSpan
	for _, t := range o.Tabs {
		label := strconv.Itoa(t.Index) + " " + t.Name
		if t.Active {
			label += g.ActiveTab
		}
		if t.Zoomed {
			label += g.Zoomed
		}
		if mark := t.Agent.glyph(g); mark != "" {
			label += " " + mark
		}
		if t.Driven {
			label += " " + g.Driven
		}
		style := tabStyle(th, t, bar)
		// The padding belongs to the tab, so the current window's block and
		// its click target are the same cells.
		label = " " + label + " "
		w := ansi.StringWidth(label)
		spans = append(spans, TabSpan{Index: t.Index, X: x, W: w})
		parts = append(parts, style.Render(label))
		x += w
	}
	left := strings.Join(parts, "")
	right := o.Right
	if o.Message != "" {
		right = o.Message
	}
	right = " " + right + " "
	lw, rw := ansi.StringWidth(left), ansi.StringWidth(right)
	if o.Message != "" {
		rw++ // the divider StatusBar draws before the message
	}
	leftMax := o.Width
	switch {
	case lw+rw <= o.Width:
	case o.Message != "" && rw < o.Width:
		// A refusal is the only feedback the user gets, so the tabs make
		// room for it.
		leftMax = o.Width - rw
	case o.Message != "":
		// Too wide even alone: cut it, and give it the whole bar but the
		// divider.
		right = " " + ansi.Truncate(o.Message, max(o.Width-3, 0), "…") + " "
		leftMax = max(o.Width-ansi.StringWidth(right)-1, 0)
	default:
		right = ""
	}
	return statusLayout{left: left, spans: spans, right: right, leftMax: leftMax}
}
