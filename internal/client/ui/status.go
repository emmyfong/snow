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
	Notice  string // shown in place of Right, in the bar's color; a Message wins
	Prompt  string // a question for the user; takes the whole bar
	Hint    string // a faint reminder before Right; dropped first when the bar is short
}

// StatusBar draws one line of exactly Width cells: the session name and the
// window tabs on the left, and Hint and Right, or Message, or Notice, on the
// right. When the line is too short, Hint is dropped first, then Right; but a
// Message or a Notice stays and the tabs are cut to make room for it; one
// wider than the line is cut too. A Prompt replaces the whole bar.
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

// SessionSpan returns the columns the session name covers in the bar
// StatusBar draws for o, and whether any of it shows. A bar cut short clips
// the name like a tab, and a Prompt takes the whole bar, so it leaves none.
func SessionSpan(th theme.Theme, o StatusOptions) (x, w int, ok bool) {
	if o.Width <= 0 || o.Prompt != "" {
		return 0, 0, false
	}
	l := layoutStatus(th, o)
	w = min(l.sessionW, l.leftMax)
	return 0, w, w > 0
}

// statusLayout is the bar's pieces before they are cut to the width.
type statusLayout struct {
	left     string    // the session and tabs, untruncated
	sessionW int       // the width of the session name's block
	spans    []TabSpan // each tab's columns inside left
	hint     string    // the faint reminder, with its padding; "" when dropped
	right    string    // with its padding; "" when dropped
	leftMax  int       // how much of left fits
}

func layoutStatus(th theme.Theme, o StatusOptions) statusLayout {
	bar := lipgloss.NewStyle().Foreground(th.BarFG).Background(th.BarBG)
	g := th.Glyphs
	first := bar.Render(" " + g.SessionLeft + o.Session + g.SessionRight + " ")
	parts := []string{first}
	x := ansi.StringWidth(first)
	sessionW := x
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
	shown := o.Message
	if shown == "" {
		shown = o.Notice
	}
	if shown != "" {
		right = shown
	}
	right = " " + right + " "
	hint := ""
	if o.Hint != "" && shown == "" {
		hint = o.Hint + "  "
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
		// The hint goes first: a window tab or Right matters more.
		hint = ""
	case shown != "" && rw < o.Width:
		// A refusal or a notice is the only feedback the user gets, so the
		// tabs make room for it.
		leftMax = o.Width - rw
	case shown != "":
		// Too wide even alone: cut it, and give it the whole bar but the
		// divider.
		right = " " + ansi.Truncate(shown, max(o.Width-2-divider, 0), "…") + " "
		leftMax = max(o.Width-ansi.StringWidth(right)-divider, 0)
	default:
		// Too wide for Right too: drop it, and the hint with it.
		right, hint = "", ""
	}
	return statusLayout{left: left, sessionW: sessionW, spans: spans, hint: hint, right: right, leftMax: leftMax}
}
