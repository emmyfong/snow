package copymode

// clamp keeps the view and the cursor inside the history, and the cursor
// inside the view.
func (m *Model) clamp() {
	if m.total == 0 {
		m.top, m.cur = 0, pos{}
		return
	}
	m.top = min(max(m.top, 0), m.bottom())
	m.cur.line = min(max(m.cur.line, 0), m.total-1)
	m.settle()
	m.reveal()
	m.top = min(max(m.top, 0), m.bottom())
}

// reveal scrolls the view just far enough to show the cursor's line.
func (m *Model) reveal() {
	switch {
	case m.cur.line < m.top:
		m.top = m.cur.line
	case m.cur.line >= m.top+m.rows:
		m.top = m.cur.line - m.rows + 1
	}
}

// moveLines moves the cursor by n lines, scrolling when it leaves the view.
func (m *Model) moveLines(n int) {
	m.cur.line = min(max(m.cur.line+n, 0), max(m.total-1, 0))
	m.reveal()
	m.settle()
}

// moveCol moves the cursor by n cells within its line. The cursor never
// rests on the second cell of a wide character: moving right it steps over
// it, moving left it lands on the character's first cell.
func (m *Model) moveCol(n int) {
	col := min(max(m.cur.col+n, 0), max(m.width(m.cur.line)-1, 0))
	if start, w := m.charAt(m.cur.line, col); start != col && n > 0 && start+w < m.width(m.cur.line) {
		col = start + w
	}
	m.cur.col = m.snap(m.cur.line, col)
	m.want = m.cur.col
}

// settle puts the cursor back on its wanted column, as far as the line goes.
func (m *Model) settle() {
	m.cur.col = m.snap(m.cur.line, min(m.want, max(m.width(m.cur.line)-1, 0)))
}

// snap returns the first cell of the character at cell col of line i, so a
// position never splits a wide character.
func (m *Model) snap(i, col int) int {
	start, _ := m.charAt(i, col)
	return start
}

// page scrolls the view by n lines and moves the cursor with it; at either
// end the cursor still moves by the full distance, so repeated pages reach
// the first and last line.
func (m *Model) page(n int) {
	m.top = min(max(m.top+n, 0), m.bottom())
	m.cur.line = min(max(m.cur.line+n, 0), max(m.total-1, 0))
	m.cur.line = min(max(m.cur.line, m.top), m.top+m.rows-1)
	m.cur.line = min(m.cur.line, max(m.total-1, 0))
	m.settle()
}

// home moves to the first line.
func (m *Model) home() {
	m.top, m.cur.line = 0, 0
	m.settle()
}

// end moves to the last line.
func (m *Model) end() {
	m.top, m.cur.line = m.bottom(), max(m.total-1, 0)
	m.settle()
}
