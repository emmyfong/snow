package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

// TextAreaOptions configures TextArea and TextAreaCursor.
type TextAreaOptions struct {
	Width  int
	Height int    // the most rows to show; at least 1
	Prompt string // drawn before the first row, in the accent color
	Value  string
	// Cursor is where typing goes, as a count of runes into Value.
	Cursor      int
	Placeholder string // shown, muted, while Value is empty
}

// TextArea draws a text field of several rows, each exactly Width cells
// wide: Value wrapped to the width after Prompt, with its line breaks, and
// scrolled so the row with the cursor shows. It draws no cursor;
// TextAreaCursor says where the terminal's cursor goes.
func TextArea(th theme.Theme, o TextAreaOptions) string {
	if o.Width <= 0 {
		return ""
	}
	prompt := ansi.Truncate(PlainText(o.Prompt), o.Width, "")
	pw := ansi.StringWidth(prompt)
	lead := lipgloss.NewStyle().Foreground(th.Accent).Render(prompt)
	pad := func(s string) string { return s + strings.Repeat(" ", max(o.Width-ansi.StringWidth(s), 0)) }
	if o.Value == "" {
		ph := ansi.Truncate(PlainText(o.Placeholder), max(o.Width-pw, 0), "")
		return pad(lead + lipgloss.NewStyle().Foreground(th.Muted).Render(ph))
	}
	rows, _, first := textRows(o)
	rows = rows[:min(first+max(o.Height, 1), len(rows))]
	out := make([]string, 0, len(rows))
	for i, r := range rows[first:] {
		l := strings.Repeat(" ", pw)
		if first+i == 0 {
			l = lead
		}
		out = append(out, pad(l+r.text))
	}
	return strings.Join(out, "\n")
}

// TextAreaCursor returns the cell of the cursor in what TextArea draws
// for o, counted from its top-left corner.
func TextAreaCursor(o TextAreaOptions) (x, y int) {
	pw := ansi.StringWidth(ansi.Truncate(PlainText(o.Prompt), max(o.Width, 0), ""))
	if o.Value == "" {
		return min(pw, max(o.Width-1, 0)), 0
	}
	rows, at, first := textRows(o)
	r := rows[at]
	col := ansi.StringWidth(string([]rune(r.text)[:o.clampCursor()-r.start]))
	return min(pw+col, max(o.Width-1, 0)), at - first
}

// TextAreaRows is how many rows TextArea draws for o.
func TextAreaRows(o TextAreaOptions) int {
	if o.Width <= 0 {
		return 0
	}
	if o.Value == "" {
		return 1
	}
	rows, _, first := textRows(o)
	return min(len(rows)-first, max(o.Height, 1))
}

type textRow struct {
	text  string
	start int // the rune index of the row's first rune in Value
}

func (o TextAreaOptions) clampCursor() int {
	return min(max(o.Cursor, 0), len([]rune(o.Value)))
}

// textRows wraps o.Value into rows of the room after the prompt, breaking
// inside words, so each rune keeps one place. It returns the rows, the row
// of the cursor, and the first row to show.
func textRows(o TextAreaOptions) (rows []textRow, at, first int) {
	room := max(o.Width-ansi.StringWidth(ansi.Truncate(PlainText(o.Prompt), o.Width, "")), 1)
	runes := []rune(o.Value)
	cur := o.clampCursor()
	var b strings.Builder
	start, width := 0, 0
	flush := func(next int) {
		rows = append(rows, textRow{text: b.String(), start: start})
		b.Reset()
		start, width = next, 0
	}
	for i, r := range runes {
		if r == '\n' {
			flush(i + 1)
			continue
		}
		s := PlainText(string(r))
		w := ansi.StringWidth(s)
		if width+w > room {
			flush(i)
		}
		b.WriteString(s)
		width += w
	}
	full := width == room
	flush(len(runes))
	if full && cur == len(runes) {
		// The cursor after a full last row goes to a row of its own.
		rows = append(rows, textRow{start: len(runes)})
	}
	for i, r := range rows {
		if cur >= r.start {
			at = i
		}
	}
	height := max(o.Height, 1)
	first = max(0, min(at-height+1, len(rows)-height))
	if at < first {
		first = at
	}
	return rows, at, first
}
