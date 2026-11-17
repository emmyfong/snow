package ui

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

// WindowBarOptions configures the window bar above the panes.
type WindowBarOptions struct {
	Width    int
	Session  string
	Tabs     []Tab
	NeedsYou int
}

// WindowBar draws the window controls in exactly Width cells.
func WindowBar(th theme.Theme, o WindowBarOptions) string {
	row, _ := WindowBarRow(th, o)
	return row
}

type windowPart struct {
	barPart
	kind  string
	index int
}

// tabLabel is shared by window tabs and their measured hit targets.
func tabLabel(th theme.Theme, t Tab) string {
	label := strconv.Itoa(t.Index) + " " + PlainLine(t.Name)
	if t.Active {
		label += th.Glyphs.ActiveTab
	}
	if t.Zoomed {
		label += th.Glyphs.Zoomed
	}
	if mark := t.Agent.glyph(th.Glyphs); mark != "" {
		label += " " + mark
	}
	if t.Driven {
		label += " " + th.Glyphs.Driven
	}
	return " " + label + " "
}

// WindowBarRow draws the window bar and reports its visible click targets.
// When short, it drops the badge, cuts the session name, then cuts the tabs.
// The new-window and menu controls keep their cells on supported widths.
func WindowBarRow(th theme.Theme, o WindowBarOptions) (string, []TopBarSpan) {
	if o.Width <= 0 {
		return "", nil
	}
	bar := lipgloss.NewStyle().Foreground(th.BarFG).Background(th.BarBG)
	button := bar.Bold(true)
	needs := lipgloss.NewStyle().Foreground(th.PromptFG).Background(th.PromptBG).Bold(true)
	session := windowPart{barPart: barPart{text: " " + PlainLine(o.Session) + " " + th.Glyphs.Dropdown + " ", style: button}, kind: TopBarSession}
	var tabs []windowPart
	for _, t := range o.Tabs {
		tabs = append(tabs, windowPart{barPart: barPart{text: tabLabel(th, t), style: tabStyle(th, t, bar)}, kind: TopBarTab, index: t.Index})
	}
	add := windowPart{barPart: barPart{text: " " + th.Glyphs.Plus + " ", style: button}, kind: TopBarNewWindow}
	menu := windowPart{barPart: barPart{text: " " + th.Glyphs.Menu + " ", style: button}, kind: TopBarMenu}
	badge := windowPart{barPart: barPart{style: needs}, kind: TopBarNeedsYou}
	if o.NeedsYou > 0 {
		badge.text = " " + strconv.Itoa(o.NeedsYou) + " need you "
	}
	tabsWidth := 0
	for _, p := range tabs {
		tabsWidth += ansi.StringWidth(p.text)
	}
	fixed := ansi.StringWidth(add.text) + ansi.StringWidth(menu.text)
	sw := ansi.StringWidth(session.text)
	if sw+tabsWidth+fixed+ansi.StringWidth(badge.text) > o.Width {
		badge.text = ""
	}
	if sw+tabsWidth+fixed > o.Width {
		keep := max(o.Width-tabsWidth-fixed, 0)
		// Keep the dropdown reachable after the session's name no longer fits.
		keep = max(keep, min(3, max(o.Width-fixed, 0)))
		if keep >= 3 {
			session.text = " " + ansi.Truncate(PlainLine(o.Session), keep-3, "…") + " " + th.Glyphs.Dropdown + " "
			session.text = ansi.Truncate(session.text, keep, "")
		} else {
			session.text = ansi.Truncate(session.text, keep, "")
		}
	}
	leftBudget := max(o.Width-fixed-ansi.StringWidth(badge.text), 0)
	parts := []windowPart{session}
	used := ansi.StringWidth(session.text)
	for _, p := range tabs {
		room := max(leftBudget-used, 0)
		if room == 0 {
			break
		}
		p.text = ansi.Truncate(p.text, room, "…")
		used += ansi.StringWidth(p.text)
		parts = append(parts, p)
	}
	parts = append(parts, add)
	rightStart := len(parts)
	if badge.text != "" {
		parts = append(parts, badge)
	}
	parts = append(parts, menu)
	// Below six cells the padded controls cannot fit. Keep the glyphs first.
	if fixed > o.Width {
		parts = []windowPart{add, menu}
		parts[0].text = th.Glyphs.Plus
		parts[1].text = th.Glyphs.Menu
		rightStart = 1
	}
	total := 0
	for _, p := range parts {
		total += ansi.StringWidth(p.text)
	}
	gap := max(o.Width-total, 0)
	var out strings.Builder
	var spans []TopBarSpan
	x := 0
	for i, p := range parts {
		if i == rightStart && gap > 0 {
			out.WriteString(bar.Render(strings.Repeat(" ", gap)))
			x += gap
		}
		text := ansi.Truncate(p.text, max(o.Width-x, 0), "")
		w := ansi.StringWidth(text)
		if w == 0 {
			continue
		}
		out.WriteString(p.style.Render(text))
		spans = append(spans, TopBarSpan{Kind: p.kind, X: x, W: w, Index: p.index})
		x += w
	}
	if x < o.Width {
		out.WriteString(bar.Render(strings.Repeat(" ", o.Width-x)))
	}
	return out.String(), spans
}
