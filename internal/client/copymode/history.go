package copymode

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/pkg/api"
)

// ahead and behind are how many screens of history around the view are
// fetched, so ordinary scrolling never waits for the server.
const (
	behind = 2
	ahead  = 3
)

func (m *Model) request(from, count int) {
	m.flight[from] = count
	m.o.Send(&api.ScrollbackRequest{Pane: m.o.Pane, From: from, Count: count})
}

// requested reports whether line i is part of a page not yet answered.
func (m *Model) requested(i int) bool {
	for from, n := range m.flight {
		if i >= from && i < from+n {
			return true
		}
	}
	return false
}

// fill asks for the lines around the view that are not here yet.
func (m *Model) fill() {
	m.fetch(max(m.top-behind*m.rows, 0), min(m.top+ahead*m.rows, m.total))
}

// fetch asks for the lines from lo up to hi that are not here yet, one
// request per run of missing lines.
func (m *Model) fetch(lo, hi int) {
	run := -1
	flush := func(end int) {
		for run >= 0 && run < end {
			n := min(end-run, api.MaxScrollbackLines)
			m.request(run, n)
			run += n
		}
		run = -1
	}
	for i := lo; i < hi; i++ {
		_, have := m.lines[i]
		if have || m.requested(i) {
			flush(i)
			continue
		}
		if run < 0 {
			run = i
		}
	}
	flush(hi)
}

// Receive applies a server message for this pane: a page of history, or the
// result of a search. A page can finish a copy that was waiting for lines;
// then it returns Exited and the command that copies.
func (m *Model) Receive(msg api.Message) (Result, tea.Cmd) {
	if res, ok := msg.(*api.SearchResult); ok {
		m.receiveSearch(res)
		return Open, nil
	}
	r, ok := msg.(*api.Scrollback)
	if !ok || r.Pane != m.o.Pane {
		return Open, nil
	}
	delete(m.flight, r.From)
	for i, l := range r.Lines {
		m.lines[r.From+i] = l
		delete(m.plain, r.From+i)
	}
	m.total = r.Total
	if !m.ready {
		m.open()
	}
	m.clamp()
	m.fill()
	if m.copying {
		return m.copySelection()
	}
	return Open, nil
}

// open places the view and the cursor once the history's size is known.
func (m *Model) open() {
	m.ready = true
	m.top = m.bottom()
	screen := len(m.o.Screen)
	if screen == 0 {
		screen = m.rows
	}
	m.cur = pos{line: m.total - screen + m.o.CursorY, col: m.o.CursorX}
	m.want = m.o.CursorX
}
