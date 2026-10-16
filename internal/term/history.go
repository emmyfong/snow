package term

import "strings"

// History returns up to n lines of the pane's history from line from, with
// styles as ANSI escape sequences, and the history's length. The history is
// the scrollback, oldest first, followed by the rows now on the screen, so
// line 0 is the oldest kept line. A request past the end returns no lines.
// Dropped counts the lines that fell off the top of a full scrollback since
// the pane started: when it grows by d, every line number shifts down by d.
// It may overcount, never undercount. All three are read under one lock, so
// they agree.
func (p *Pane) History(from, n int) (lines []string, total, dropped int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	sb := p.emu.Scrollback()
	kept := sb.Len()
	total = kept + p.emu.Height()
	dropped = p.shift.dropped
	if from < 0 || from >= total || n <= 0 {
		return nil, total, dropped
	}
	end := min(from+n, total)
	lines = make([]string, 0, end-from)
	for i := from; i < min(end, kept); i++ {
		lines = append(lines, sb.Line(i).Render())
	}
	if end > kept {
		rows := p.screenLines()
		lines = append(lines, rows[max(from-kept, 0):end-kept]...)
	}
	return lines, total, dropped
}

// screenLines returns the screen as one styled string per row. The caller
// holds mu.
func (p *Pane) screenLines() []string {
	rows := p.emu.Height()
	lines := strings.SplitN(p.emu.Render(), "\n", rows)
	for len(lines) < rows {
		lines = append(lines, "")
	}
	return lines
}
