package copymode

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/ui"
)

// toggleSelection starts a selection at the cursor, or cancels the one that
// runs.
func (m *Model) toggleSelection() {
	m.selecting = !m.selecting
	m.anchor = m.cur
}

// span returns the selection's first and last cell, in history order, and
// whether there is a selection. The last cell is included.
func (m *Model) span() (from, to pos, ok bool) {
	if !m.selecting {
		return pos{}, pos{}, false
	}
	from, to = m.anchor, m.cur
	if to.line < from.line || (to.line == from.line && to.col < from.col) {
		from, to = to, from
	}
	return from, to, true
}

// copySelection copies the selected text and leaves copy mode. When lines
// of the selection have not arrived, it asks for them and finishes when they
// do.
func (m *Model) copySelection() (Result, tea.Cmd) {
	from, to, ok := m.span()
	if !ok {
		m.copying = false
		m.notice = "Nothing selected"
		return Open, nil
	}
	m.copying = false
	for i := from.line; i <= to.line; i++ {
		if _, have := m.lines[i]; !have {
			m.copying = true
			m.fetch(from.line, to.line+1)
			return Open, nil
		}
	}
	text := selectionText(m.text, from, to)
	m.notice = fmt.Sprintf("Copied %d characters", len([]rune(text)))
	if m.o.Copy == nil {
		return Exited, nil
	}
	return Exited, m.o.Copy(text)
}

// selectionText returns the text from cell from to cell to, both included,
// of the lines that line(i) returns. Trailing spaces are cut from each line,
// and lines are joined with newlines. The terminal does not say which line
// breaks are soft wraps, so a wrapped line comes out as several.
func selectionText(line func(int) string, from, to pos) string {
	var out []string
	for i := from.line; i <= to.line; i++ {
		s := line(i)
		lo, hi := 0, ansi.StringWidth(s)
		if i == from.line {
			lo = from.col
		}
		if i == to.line {
			hi = to.col + 1
		}
		out = append(out, strings.TrimRight(cells(s, lo, hi), " "))
	}
	return strings.Join(out, "\n")
}

// cells returns the characters of s that start in cells lo up to hi. A wide
// character starts in its first cell, so it is kept whole or left out.
func cells(s string, lo, hi int) string {
	var b strings.Builder
	col, keep := 0, false
	for _, r := range s {
		if w := ansi.StringWidth(string(r)); w > 0 {
			keep = col >= lo && col < hi
			col += w
		}
		if keep {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// selection returns the highlights for the selected cells in the view.
func (m *Model) selection() []ui.Highlight {
	from, to, ok := m.span()
	if !ok {
		return nil
	}
	var out []ui.Highlight
	for row := range m.rows {
		i := m.top + row
		if i < from.line || i > to.line {
			continue
		}
		w := m.width(i)
		lo, hi := 0, w
		if i == from.line {
			lo = from.col
		}
		if i == to.line {
			hi = to.col + m.cellWidth(i, to.col)
		}
		// An empty line, or a selection ending past its text, still shows
		// one cell, so the user sees the line is part of it.
		out = append(out, ui.Highlight{Row: row, Col: lo, Len: max(hi-lo, 1), Kind: ui.HighlightSelected})
	}
	return out
}

// cellWidth returns the width of the character at cell col of line i: 2 for
// a wide character, otherwise 1.
func (m *Model) cellWidth(i, col int) int {
	s := m.text(i)
	c := 0
	for _, r := range s {
		w := ansi.StringWidth(string(r))
		if w > 0 && c <= col && col < c+w {
			return w
		}
		c += w
	}
	return 1
}
