package home

import (
	"slices"

	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// placement is where the page's parts go for the current size, shared by
// drawing and by clicks.
type placement struct {
	header   bool // wordmark and version fit
	top      int  // first row of the page
	x, width int  // the column and width of the input, lists, and hints
	list     band // the main list: themes, commands, or sessions
	sessions band // the sessions under the command list; empty otherwise
}

// band is where one list's rows go.
type band struct {
	top, lines   int // the first row and the rows set aside
	first, count int // index of the first item shown, and items shown
}

// Rows of each part. The header is the wordmark, the version, and a blank.
const (
	headerRows = ui.WordmarkHeight + 2
	inputRows  = 3                         // a framed one-line input
	fixedRows  = inputRows + 1 + 1 + 1 + 1 // input, blank, label, blank, hints
	bandRows   = 2                         // a blank and the label above a band
)

// layout places the page in the screen: centered, the list as tall as the
// room allows, the wordmark dropped first when the screen is short. While a
// command is typed, the sessions follow the commands and shrink first.
func (m *Model) layout() placement {
	p := placement{width: min(64, max(m.cols-4, 0))}
	p.x = max((m.cols-p.width)/2, 0)
	shown, selected := m.listLen(), m.listSel()
	notes := len(m.noteLines())
	room := m.rows - fixedRows - notes
	p.header = m.cols >= ui.WordmarkWidth && room-headerRows >= min(max(shown, 1), 3)
	if p.header {
		room -= headerRows
	}
	p.list.lines = min(max(shown, 1), max(room, 1))
	total := fixedRows + p.list.lines + notes
	if p.header {
		total += headerRows
	}
	if m.commanding() && !m.picking {
		if n := min(max(len(m.sessions), 1), room-p.list.lines-bandRows); n > 0 {
			p.sessions.lines = n
			total += bandRows + n
		}
	}
	p.top = max((m.rows-total)/2, 0)
	p.list.top = p.top + inputRows + 2
	if p.header {
		p.list.top += headerRows
	}
	p.list.count = min(shown, p.list.lines)
	if selected >= p.list.count {
		p.list.first = selected - p.list.count + 1
	}
	if b := &p.sessions; b.lines > 0 {
		b.top = p.list.top + p.list.lines + bandRows
		b.count = min(len(m.sessions), b.lines)
		if t := m.targetIndex(); t >= b.count {
			b.first = t - b.count + 1
		}
	}
	return p
}

// targetIndex is the index in the sessions of the session a command acts
// on, or -1.
func (m *Model) targetIndex() int {
	return slices.IndexFunc(m.sessions, func(s api.SessionInfo) bool { return s.Name == m.target })
}

// listLen and listSel are the list's length and selected row: the matching
// commands while a slash command is typed, else the sessions.
func (m *Model) listLen() int {
	if m.picking {
		return len(theme.Builtin())
	}
	if m.commanding() {
		return len(m.matches())
	}
	return len(m.shown())
}

func (m *Model) listSel() int {
	if m.picking {
		return m.themeSel
	}
	if m.commanding() {
		return m.cmdSel
	}
	if m.naming != nil {
		return slices.IndexFunc(m.sessions, func(s api.SessionInfo) bool { return s.Name == m.naming.session })
	}
	return m.selected
}

// rowAt returns the index of the item on screen row y.
func (b band) rowAt(y int) (int, bool) {
	if y < b.top || y >= b.top+b.count {
		return 0, false
	}
	return b.first + y - b.top, true
}
