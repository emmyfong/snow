package copymode

// rebase brings what copy mode holds in step with a reply's numbering. A
// growing dropped count means the pane printed with a full scrollback: every
// line number moved down by the difference, so cached lines, the cursor, the
// selection and the matches move with them. When the history changed at all,
// the rows that were on the pane's screen may hold new text, so they are
// fetched again. The first reply only sets the base.
func (m *Model) rebase(dropped, total int) {
	if !m.based {
		m.based, m.dropped = true, dropped
		return
	}
	d := dropped - m.dropped
	if d <= 0 && total == m.total {
		return
	}
	d = max(d, 0)
	m.dropped += d
	screen := m.total - m.rows // the first line that was on the screen
	m.lines = shiftLines(m.lines, d, screen)
	m.plain = shiftLines(m.plain, d, screen)
	// Pages in flight were asked for in the old numbering; their answers
	// still arrive and are stored, and fill asks again for any gap.
	m.flight = map[int]int{}
	m.cur.line -= d
	m.anchor.line -= d
	m.top -= d
	if m.cur.line < 0 || m.anchor.line < 0 {
		m.selecting = false
	}
	m.cur.line, m.anchor.line, m.top = max(m.cur.line, 0), max(m.anchor.line, 0), max(m.top, 0)
	m.shiftMatches(d)
}

// shiftLines moves the lines below screen up by d and drops the rest.
func shiftLines(lines map[int]string, d, screen int) map[int]string {
	out := make(map[int]string, len(lines))
	for i, l := range lines {
		if i < screen && i-d >= 0 {
			out[i-d] = l
		}
	}
	return out
}

// shiftMatches moves the search matches up by d lines and drops those that
// fell off the top.
func (m *Model) shiftMatches(d int) {
	if d == 0 || len(m.search.matches) == 0 {
		return
	}
	kept := m.search.matches[:0:0]
	gone := 0
	for _, mt := range m.search.matches {
		if mt.Line-d < 0 {
			gone++
			continue
		}
		mt.Line -= d
		kept = append(kept, mt)
	}
	m.search.matches = kept
	if m.search.current >= 0 {
		m.search.current -= gone
		if m.search.current < 0 {
			m.search.current = -1
		}
	}
}
