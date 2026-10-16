package term

import (
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
)

// lineShift counts the history lines that fell off the top of a full
// scrollback. vt does not report them, so before each change it marks the
// newest non-empty scrollback line and finds it again after: vt copies each
// line into its own allocation, and the mark keeps that alive, so the
// address names the line.
type lineShift struct {
	dropped int      // lines dropped so far; at least the true count
	mark    *uv.Cell // first cell of the newest non-empty line, or nil
	at      int      // the marked line's index
	length  int      // the scrollback's length when marked
}

// before marks sb ahead of a change. The caller holds the pane's mu.
func (s *lineShift) before(sb *vt.Scrollback) {
	s.mark, s.length = nil, sb.Len()
	for i := sb.Len() - 1; i >= 0; i-- {
		if l := sb.Line(i); len(l) > 0 {
			s.mark, s.at = &l[0], i
			return
		}
	}
}

// after counts the lines dropped since before. When the mark is gone, or
// there was none and the scrollback is now full, the count is unknown; it
// then grows by more than every line that was kept, so a client that
// compares two counts drops everything it had.
func (s *lineShift) after(sb *vt.Scrollback) {
	n := sb.Len()
	if s.mark == nil {
		if n >= sb.MaxLines() && n > s.length {
			s.dropped += s.length + n
		}
		return
	}
	for i := min(s.at, n-1); i >= 0; i-- {
		if l := sb.Line(i); len(l) > 0 && &l[0] == s.mark {
			s.dropped += s.at - i
			return
		}
	}
	s.dropped += s.length + n
}
