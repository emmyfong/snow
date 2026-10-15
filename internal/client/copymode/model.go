// Package copymode is the screen that reads a pane's history: it scrolls the
// scrollback and the screen, searches them, and copies a selection to the
// clipboard. The session screen opens it over the focused pane and routes
// keys and server messages to it, as it does for the picker.
package copymode

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/pkg/api"
)

// Result is what an event did to copy mode.
type Result int

// Results.
const (
	Open   Result = iota // copy mode stays open
	Exited               // the user left copy mode
)

// Options configures New.
type Options struct {
	Pane       int               // the pane whose history is shown
	Cols, Rows int               // the pane's size in cells
	Screen     []string          // the pane's rows now, shown until the history arrives
	CursorX    int               // the pane's cursor, where copy mode's cursor starts
	CursorY    int               //
	Send       func(api.Message) // sends a request to the server, in order
}

// pos is a place in the history: a line number and a cell column.
type pos struct{ line, col int }

// Model is one open copy mode. It shows the history as lines numbered from
// 0, the oldest, as the protocol does, and keeps the pages it has fetched.
type Model struct {
	o      Options
	cols   int
	rows   int
	ready  bool // the first Scrollback arrived, so total is known
	total  int
	top    int // the first visible line
	cur    pos
	want   int // the column the cursor returns to on shorter lines
	lines  map[int]string
	plain  map[int]string
	flight map[int]int // request start line → count, for pages not yet answered
	search searching
	notice string // a note for the status bar; cleared by the next key
}

// New opens copy mode and asks the server for the history's size. The cursor
// starts where the pane's cursor is, and the view at the bottom, so copy mode
// opens on what was on screen.
func New(o Options) *Model {
	m := &Model{o: o, cols: o.Cols, rows: o.Rows, lines: map[int]string{}, plain: map[int]string{}, flight: map[int]int{}}
	m.request(0, 1)
	return m
}

// Pane returns the pane whose history is shown.
func (m *Model) Pane() int { return m.o.Pane }

// Resize sets the view's size in cells and keeps the cursor on screen.
func (m *Model) Resize(cols, rows int) {
	m.cols, m.rows = cols, rows
	if m.ready {
		m.clamp()
		m.fill()
	}
}

// bottom returns the largest first line the view can have.
func (m *Model) bottom() int { return max(m.total-m.rows, 0) }

// text returns line i without styles or trailing spaces, or "" while the
// line has not arrived.
func (m *Model) text(i int) string {
	if p, ok := m.plain[i]; ok {
		return p
	}
	l, ok := m.lines[i]
	if !ok {
		return ""
	}
	p := strings.TrimRight(ansi.Strip(l), " ")
	m.plain[i] = p
	return p
}

// width returns line i's width in cells.
func (m *Model) width(i int) int { return ansi.StringWidth(m.text(i)) }
