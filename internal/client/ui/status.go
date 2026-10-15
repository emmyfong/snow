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
}

// StatusOptions configures StatusBar.
type StatusOptions struct {
	Width   int
	Session string
	Tabs    []Tab
	Right   string // for example the pane's profile and the time
	Message string // shown in place of Right, in the error color
}

// StatusBar draws one line of exactly Width cells: the session name and the
// window tabs on the left, and Right, or Message, on the right. When the line
// is too short, Right is dropped first, but a Message stays and the tabs are
// cut to make room for it.
func StatusBar(th theme.Theme, o StatusOptions) string {
	if o.Width <= 0 {
		return ""
	}
	bar := lipgloss.NewStyle().Foreground(th.BarFG).Background(th.BarBG)
	active := bar.Foreground(th.TabActive).Bold(true)
	g := th.Glyphs
	parts := []string{bar.Render(" " + g.SessionLeft + o.Session + g.SessionRight)}
	for _, t := range o.Tabs {
		label := strconv.Itoa(t.Index) + ":" + t.Name
		if t.Active {
			label += g.ActiveTab
		}
		if t.Zoomed {
			label += g.Zoomed
		}
		style := bar
		if t.Active {
			style = active
		}
		parts = append(parts, bar.Render(" ")+style.Render(label))
	}
	left := strings.Join(parts, "")
	right, rightStyle := o.Right, bar
	if o.Message != "" {
		right, rightStyle = o.Message, bar.Foreground(th.Error).Bold(true)
	}
	right = " " + right + " "
	lw, rw := ansi.StringWidth(left), ansi.StringWidth(right)
	switch {
	case lw+rw <= o.Width:
	case o.Message != "" && rw < o.Width:
		// A refusal is the only feedback the user gets, so the tabs make
		// room for it.
		left = ansi.Truncate(left, o.Width-rw, "")
	default:
		right, rw = "", 0
	}
	left = ansi.Truncate(left, o.Width, "")
	gap := max(o.Width-ansi.StringWidth(left)-rw, 0)
	return left + bar.Render(strings.Repeat(" ", gap)) + rightStyle.Render(right)
}
