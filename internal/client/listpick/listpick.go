// Package listpick is a popup that lists rows and lets the user pick one:
// the base of the mode and resume pickers. It owns the selection, the keys,
// the click mapping, and the drawing; a feature package gives it rows and
// reads the choice.
package listpick

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
)

const (
	maxWidth = 100
	minWidth = 28
	maxRows  = 12
	// chrome is the rows the popup spends beside the list: two frame edges,
	// a blank row, and the key hints.
	chrome = 4
	// page is how many rows PgUp and PgDn move.
	page = 5
)

// Model is one open list popup.
type Model struct {
	title string
	hints []ui.Hint
	items []ui.ListItem
	sel   int
	note  string // shown in place of the rows while there are none
}

// New returns a popup titled title with key hints in its bottom row.
func New(title string, hints []ui.Hint) *Model {
	return &Model{title: title, hints: hints}
}

// SetTitle changes the popup's title.
func (m *Model) SetTitle(title string) { m.title = title }

// SetHints replaces the key hints in the bottom row.
func (m *Model) SetHints(h []ui.Hint) { m.hints = h }

// SetNote sets the line shown while the popup has no rows, for example
// "Loading..." or "Nothing saved yet".
func (m *Model) SetNote(note string) { m.note = note }

// SetRows replaces the rows; each row is its columns, left to right. The
// selection stays on its index, kept in range.
func (m *Model) SetRows(rows [][]string) {
	m.items = make([]ui.ListItem, len(rows))
	for i, cols := range rows {
		m.items[i] = ui.ListItem{Cols: cols}
	}
	m.sel = min(m.sel, max(len(rows)-1, 0))
}

// Select moves the selection to row i, kept in range.
func (m *Model) Select(i int) { m.sel = min(max(i, 0), max(len(m.items)-1, 0)) }

// Selected returns the index of the selected row, or -1 with no rows.
func (m *Model) Selected() int {
	if len(m.items) == 0 {
		return -1
	}
	return m.sel
}

// Len returns the number of rows.
func (m *Model) Len() int { return len(m.items) }

// Key handles one key press: arrows, j and k, PgUp, PgDn, Home, and End
// move; Enter accepts the selected row; Esc cancels.
func (m *Model) Key(k tea.Key) picker.Result {
	n := len(m.items)
	switch {
	case k.Code == tea.KeyEscape:
		return picker.Cancelled
	case k.Code == tea.KeyEnter:
		if n == 0 {
			return picker.Cancelled
		}
		return picker.Accepted
	case n == 0:
	case k.Code == tea.KeyUp || k.Text == "k":
		m.sel = picker.Move(m.sel, -1, n)
	case k.Code == tea.KeyDown || k.Text == "j":
		m.sel = picker.Move(m.sel, 1, n)
	case k.Code == tea.KeyPgUp:
		m.sel = max(m.sel-page, 0)
	case k.Code == tea.KeyPgDown:
		m.sel = min(m.sel+page, n-1)
	case k.Code == tea.KeyHome:
		m.sel = 0
	case k.Code == tea.KeyEnd:
		m.sel = n - 1
	}
	return picker.Open
}

// layout is the drawn popup and where its rows are, in window cells.
type layout struct {
	box        string
	x, y, w, h int
	top, n     int   // first list row and how many list rows are drawn
	index      []int // the item on each list row
}

func (m *Model) layout(th theme.Theme, cols, rows int) (layout, bool) {
	w := min(maxWidth, cols-4)
	if w < minWidth || rows < chrome+1 {
		return layout{}, false
	}
	n := min(max(len(m.items), 1), maxRows, rows-chrome)
	h := n + chrome
	x, y := max((cols-w)/2, 0), max((rows-h)/2, 0)
	inner := w - 2
	var list string
	var index []int
	if len(m.items) == 0 {
		line := " " + lipgloss.NewStyle().Foreground(th.Muted).Render(m.note)
		list = line + strings.Repeat("\n", n-1)
	} else {
		list, index = ui.ListRows(th, ui.ListOptions{Items: m.items, Selected: m.sel, Width: inner - 1, Height: n})
		list = indent(list)
	}
	body := list + "\n\n " + ui.KeyHints(th, m.hints, inner-1)
	box := ui.Panel(th, ui.PanelOptions{Title: m.title, Body: body, Width: w})
	return layout{box: box, x: x, y: y, w: w, h: h, top: y + 1, n: n, index: index}, true
}

// indent puts one blank cell before each line.
func indent(s string) string {
	return " " + strings.ReplaceAll(s, "\n", "\n ")
}

// View draws the popup centered on a window of cols by rows cells and
// returns the box with the cell of its top-left corner. It returns "" when
// the window is too small; the keys still answer.
func (m *Model) View(th theme.Theme, cols, rows int) (box string, x, y int) {
	l, ok := m.layout(th, cols, rows)
	if !ok {
		return "", 0, 0
	}
	return l.box, l.x, l.y
}

// Click handles a left click at window cell x, y. A click on a row accepts
// it; a click outside the popup cancels; a click on its frame does nothing.
func (m *Model) Click(th theme.Theme, cols, rows, x, y int) picker.Result {
	l, ok := m.layout(th, cols, rows)
	if !ok || x < l.x || x >= l.x+l.w || y < l.y || y >= l.y+l.h {
		return picker.Cancelled
	}
	if r := y - l.top; r >= 0 && r < len(l.index) {
		m.sel = l.index[r]
		return picker.Accepted
	}
	return picker.Open
}
