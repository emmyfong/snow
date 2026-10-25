package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

// TopBarOptions configures TopBar. Empty parts are left out.
type TopBarOptions struct {
	Width int
	Title string
	Gauge string // how full the context is, for example "ctx 31%"
	// GaugeHot draws the gauge in Removed, when the context is nearly
	// full.
	GaugeHot bool
	Cost     string // for example "$0.42 est."
	Model    string // always shown, for example "anthropic/claude-sonnet-5-5 high"
	Mode     string // for example "mode: ask"
}

// TopBar draws a chat's top bar in exactly Width cells: the title in bold
// on the left, a rule in Border, then the gauge, the cost, the model in
// the accent color, and the mode. When the row is short it drops the
// cost, then the gauge, then the mode, then the title; the model stays,
// cut if it must be.
func TopBar(th theme.Theme, o TopBarOptions) string {
	if o.Width <= 0 {
		return ""
	}
	muted := lipgloss.NewStyle().Foreground(th.Muted)
	gauge := muted
	if o.GaugeHot {
		gauge = lipgloss.NewStyle().Foreground(th.Removed)
	}
	parts := []*barPart{
		{text: o.Gauge, style: gauge},
		{text: o.Cost, style: muted},
		{text: o.Model, style: lipgloss.NewStyle().Foreground(th.Accent).Bold(true)},
		{text: o.Mode, style: muted},
	}
	title := &barPart{text: PlainLine(o.Title), style: lipgloss.NewStyle().Bold(true)}
	for _, p := range parts {
		p.text = PlainLine(p.text)
	}
	// The order parts go when the row is short.
	drop := []*barPart{parts[1], parts[0], parts[3], title}
	for _, p := range drop {
		if barWidth(title, parts) <= o.Width {
			break
		}
		p.text = ""
	}
	var right []string
	for _, p := range parts {
		if p.text != "" {
			right = append(right, p.style.Render(p.text))
		}
	}
	rhs := strings.Join(right, "  ")
	if ansi.StringWidth(rhs) > o.Width-3 {
		// Only the model is left, and it does not fit.
		cut := " " + ansi.Truncate(rhs, o.Width-1, "")
		return cut + strings.Repeat(" ", max(o.Width-ansi.StringWidth(cut), 0))
	}
	left := " "
	if title.text != "" {
		left += title.style.Render(title.text) + " "
	}
	fill := o.Width - ansi.StringWidth(left) - ansi.StringWidth(rhs) - 2
	return left + lipgloss.NewStyle().Foreground(th.Border).Render(strings.Repeat(th.Glyphs.Horizontal, max(fill, 0))) + " " + rhs + " "
}

type barPart struct {
	text  string
	style lipgloss.Style
}

// barWidth is the width of the bar with the parts that are left, and a
// rule of at least one cell.
func barWidth(title *barPart, parts []*barPart) int {
	w := 1 + 1 + 1 + 1 // the edges, the rule, and the space after it
	if title.text != "" {
		w += ansi.StringWidth(title.text) + 1
	}
	n := 0
	for _, p := range parts {
		if p.text != "" {
			w += ansi.StringWidth(p.text)
			n++
		}
	}
	return w + 2*max(n-1, 0)
}
