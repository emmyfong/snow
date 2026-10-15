package copymode

import (
	"fmt"
	"sort"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// searching is the state of a search: the prompt the user types in, and the
// matches the server found for the last query.
type searching struct {
	typing   bool   // the prompt is open
	input    string // what the user has typed
	backward bool   // the last search runs toward older lines
	query    string // the last query sent
	waiting  bool   // the answer to query has not arrived
	matches  []api.Match
	current  int // index in matches of the match the cursor is on, or -1
}

// startSearch opens the search prompt. Backward searches toward older
// lines, which is up.
func (m *Model) startSearch(backward bool) {
	m.search.typing, m.search.input, m.search.backward = true, "", backward
}

// Prompt returns the line the user is typing a search in, or "" when no
// prompt is open. The session shows it in place of the status bar.
func (m *Model) Prompt() string {
	if !m.search.typing {
		return ""
	}
	dir := "down"
	if m.search.backward {
		dir = "up"
	}
	return fmt.Sprintf("Search %s: %s", dir, m.search.input)
}

// Status returns a short note on the last search, or "". It clears on the
// next key.
func (m *Model) Status() string { return m.notice }

// promptKey handles a key while the search prompt is open.
func (m *Model) promptKey(k tea.Key) {
	switch {
	case k.Code == tea.KeyEscape:
		m.search.typing = false
	case k.Code == tea.KeyEnter:
		m.search.typing = false
		m.submit(m.search.input)
	case k.Code == tea.KeyBackspace:
		if _, n := utf8.DecodeLastRuneInString(m.search.input); n > 0 {
			m.search.input = m.search.input[:len(m.search.input)-n]
		}
	case k.Code == 'u' && k.Mod == tea.ModCtrl:
		m.search.input = ""
	case k.Text != "" && k.Mod&^tea.ModShift == 0:
		if len(m.search.input)+len(k.Text) <= api.MaxQueryBytes {
			m.search.input += k.Text
		}
	}
}

// submit asks the server to search for query. An empty query does nothing.
func (m *Model) submit(query string) {
	if query == "" {
		return
	}
	m.search.query, m.search.waiting = query, true
	m.search.matches, m.search.current = nil, -1
	m.o.Send(&api.SearchRequest{Pane: m.o.Pane, Query: query})
}

// repeat jumps to the next match in the last search's direction, or the
// opposite one when reverse is set.
func (m *Model) repeat(reverse bool) {
	switch {
	case m.search.query == "":
		m.notice = "No previous search"
	case m.search.waiting:
	default:
		m.jump(m.search.backward != reverse)
	}
}

// receiveSearch applies a search result and jumps to the first match from
// the cursor. A result for a query the user has replaced is dropped.
func (m *Model) receiveSearch(r *api.SearchResult) {
	if r.Pane != m.o.Pane || !m.search.waiting || r.Query != m.search.query {
		return
	}
	m.search.waiting = false
	m.search.matches = r.Matches
	m.search.current = -1
	if len(r.Matches) == 0 {
		m.notice = "No match for " + r.Query
		return
	}
	m.jump(m.search.backward)
	if r.Truncated {
		m.notice = fmt.Sprintf("First %d matches only", len(r.Matches))
	}
}

// jump moves the cursor to the next match from it, backward or forward,
// wrapping past either end of the history.
func (m *Model) jump(backward bool) {
	i, wrapped := find(m.search.matches, m.cur, backward)
	if i < 0 {
		m.notice = "No match for " + m.search.query
		return
	}
	m.search.current = i
	mt := m.search.matches[i]
	m.cur = pos{line: mt.Line, col: mt.Col}
	m.want = mt.Col
	if mt.Line < m.top || mt.Line >= m.top+m.rows {
		m.top = min(max(mt.Line-m.rows/2, 0), m.bottom())
	}
	m.notice = ""
	if wrapped {
		m.notice = "Search wrapped"
	}
	m.fill()
}

// find returns the index of the match after from (or before it, going
// backward) and whether the search wrapped past the end of the history to
// reach it. Matches are in line order. It returns -1 when there are none.
func find(matches []api.Match, from pos, backward bool) (idx int, wrapped bool) {
	n := len(matches)
	if n == 0 {
		return -1, false
	}
	before := func(i int) bool { // match i starts before from
		mt := matches[i]
		return mt.Line < from.line || (mt.Line == from.line && mt.Col < from.col)
	}
	if backward {
		// The first match not before from; the one before it is the answer.
		i := sort.Search(n, func(i int) bool { return !before(i) })
		if i == 0 {
			return n - 1, true
		}
		return i - 1, false
	}
	// The first match strictly after from.
	i := sort.Search(n, func(i int) bool {
		mt := matches[i]
		return mt.Line > from.line || (mt.Line == from.line && mt.Col > from.col)
	})
	if i == n {
		return 0, true
	}
	return i, false
}

// highlights returns the search highlights for the visible rows.
func (m *Model) highlights() []ui.Highlight {
	ms := m.search.matches
	var out []ui.Highlight
	for i := sort.Search(len(ms), func(i int) bool { return ms[i].Line >= m.top }); i < len(ms) && ms[i].Line < m.top+m.rows; i++ {
		kind := ui.HighlightMatch
		if i == m.search.current {
			kind = ui.HighlightCurrent
		}
		out = append(out, ui.Highlight{Row: ms[i].Line - m.top, Col: ms[i].Col, Len: ms[i].Len, Kind: kind})
	}
	return out
}
