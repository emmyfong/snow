package gitpane

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// boxRows is the height of the commit message box.
const boxRows = 3

// boxPrompt starts the commit message box.
const boxPrompt = "Message: "

// hitKind is what a row of the pane does on a click.
type hitKind int

const (
	hitNone    hitKind = iota
	hitEntry           // a change; index is its entry
	hitHints           // the key hints
	hitBox             // the commit message box
	hitButtons         // the action buttons
)

// hint is a key hint and the key a click on it presses.
type hint struct {
	ui.Hint
	key tea.Key
}

// layout is the drawn pane and what each row does.
type layout struct {
	rows   []string
	hits   []hitKind
	index  []int // the entry on each hitEntry row
	hints  []hint
	boxTop int // the first row of the commit box, or -1
}

func (l *layout) add(row string, kind hitKind, index int) {
	l.rows = append(l.rows, row)
	l.hits = append(l.hits, kind)
	l.index = append(l.index, index)
}

// View returns the pane's rows, each at most its width.
func (m *Model) View() []string {
	if m.width <= 0 || m.height <= 0 {
		return nil
	}
	return m.layout().rows
}

// Cursor says where the terminal's cursor goes: in the commit box while it
// is open.
func (m *Model) Cursor() (x, y int, ok bool) {
	l := m.layout()
	if m.mode != modeCommit || l.boxTop < 0 {
		return 0, 0, false
	}
	cx, cy := ui.TextAreaCursor(m.boxOptions())
	return cx + 1, l.boxTop + cy, true
}

// Click handles a left click at pane cell x, y and returns the action to
// send, or nil. A click selects a change, and a click on the selected one
// shows its diff. A click on a key hint presses its key, and a click on
// the commit box opens it.
func (m *Model) Click(x, y int) api.Message {
	// Hit testing uses the layout the user saw before its note clears.
	l := m.layout()
	m.note = ""
	if !m.loaded {
		return nil
	}
	if y < 0 || y >= len(l.rows) {
		return nil
	}
	switch l.hits[y] {
	case hitEntry:
		es := m.entries()
		i := l.index[y]
		if i == m.selected(es) {
			return m.openDiff(es, i)
		}
		m.move(es, i)
	case hitHints:
		hs := make([]ui.Hint, len(l.hints))
		for i, h := range l.hints {
			hs[i] = h.Hint
		}
		// The hints row is indented by one cell.
		for _, sp := range ui.HintSpans(hs, m.width-1) {
			if x-1 >= sp.X && x-1 < sp.X+sp.W {
				return m.Key(l.hints[sp.Index].key)
			}
		}
	case hitButtons:
		opts, actions := m.buttons()
		for _, sp := range ui.ButtonRowSpans(opts) {
			if x >= sp.X && x < sp.X+sp.W {
				return m.Key(actions[sp.Index].Key)
			}
		}
	case hitBox:
		m.openCommit()
	}
	return nil
}

// Chrome rows: the header and the key hints, plus the status row and the
// commit box when they show. A pane too short for them shows the header
// and the list only.
func (m *Model) chrome() (status, box, hints bool) {
	// A read that failed shows its error in the body instead.
	status = m.removePath != "" || (m.note != "" && (!m.readFailed() || m.mode == modeCommit)) || (m.st.Error != "" && (!m.readFailed() || m.mode == modeCommit)) || m.st.Notice != ""
	box = m.mode == modeCommit
	if m.height < 4 {
		return false, false, false
	}
	if box && m.height < 3+boxRows { // header, one list row, hints, and box
		box = false
	}
	hints = m.focused
	if status {
		status = m.statusHeight(box, hints) > 0
	}
	return status, box, hints
}

// listHeight is the rows the changes get.
func (m *Model) listHeight() int {
	status, box, hints := m.chrome()
	h := m.height - 1
	if status {
		h -= m.statusHeight(box, hints)
	}
	if box {
		h -= boxRows
	}
	if hints {
		h--
	}
	return max(h, 1)
}

// statusHeight is the number of status rows that fit while keeping one list
// row and the other chrome visible.
func (m *Model) statusHeight(box, hints bool) int {
	available := m.height - 2 // header and one list row
	if box {
		available -= boxRows
	}
	if hints {
		available--
	}
	lines, _ := m.statusLines()
	return min(len(lines), max(available, 0))
}

// diffHeight is the rows the diff gets: all but the header and the hints.
func (m *Model) diffHeight() int {
	if m.height < 4 {
		return max(m.height-1, 1)
	}
	return m.height - 2
}

func (m *Model) layout() layout {
	l := layout{boxTop: -1}
	if m.width <= 0 || m.height <= 0 {
		return l
	}
	if m.mode == modeDiff && m.st.Diff != nil {
		m.diffLayout(&l)
	} else {
		m.listLayout(&l)
	}
	for len(l.rows) < m.height {
		l.add("", hitNone, 0)
	}
	l.rows, l.hits, l.index = l.rows[:m.height], l.hits[:m.height], l.index[:m.height]
	for i, r := range l.rows {
		l.rows[i] = ansi.Truncate(r, m.width, "")
	}
	return l
}
