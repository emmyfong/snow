package copymode

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/pkg/api"
)

// fakeServer plays the server's history: n lines, "line 0" to "line n-1",
// and every request it has seen.
type fakeServer struct {
	lines    []string
	requests []*api.ScrollbackRequest
	searches []*api.SearchRequest
	pending  []*api.ScrollbackRequest
	dropped  int // lines dropped off the top, as a full scrollback does
}

func numbered(n int) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}
	return lines
}

func (s *fakeServer) send(m api.Message) {
	if r, ok := m.(*api.SearchRequest); ok {
		s.searches = append(s.searches, r)
	}
	if r, ok := m.(*api.ScrollbackRequest); ok {
		s.requests = append(s.requests, r)
		s.pending = append(s.pending, r)
	}
}

// answer serves every pending request to m, as the server would, until none
// is left.
func (s *fakeServer) answer(m *Model) {
	for len(s.pending) > 0 {
		r := s.pending[0]
		s.pending = s.pending[1:]
		var page []string
		if r.From < len(s.lines) {
			page = s.lines[r.From:min(r.From+r.Count, len(s.lines))]
		}
		m.Receive(&api.Scrollback{Pane: r.Pane, From: r.From, Total: len(s.lines), Dropped: s.dropped, Lines: page})
	}
}

// print adds n new lines at the bottom and drops as many from the top, as a
// pane with a full scrollback does.
func (s *fakeServer) print(n int) {
	next := s.dropped + len(s.lines)
	for i := range n {
		s.lines = append(s.lines, fmt.Sprintf("line %d", next+i))
	}
	s.lines = s.lines[n:]
	s.dropped += n
}

// open returns copy mode over a history of n lines, 80 by 24, opened as it
// is from a pane whose screen shows the last 24 lines and whose cursor is on
// the last row.
func open(t *testing.T, n int) (*Model, *fakeServer) {
	t.Helper()
	srv := &fakeServer{lines: numbered(n)}
	screen := srv.lines[max(n-24, 0):]
	m := New(Options{Pane: 7, Cols: 80, Rows: 24, Screen: screen, CursorX: 0, CursorY: len(screen) - 1, Send: srv.send})
	srv.answer(m)
	return m, srv
}

func press(code rune, mod tea.KeyMod, text string) tea.Key {
	return tea.Key{Code: code, Mod: mod, Text: text}
}

func key(m *Model, k tea.Key) Result {
	r, _ := m.Key(k)
	return r
}

// rows returns the view's rows without styles or trailing spaces.
func rows(m *Model) []string {
	out := strings.Split(m.View(testTheme), "\n")
	for i := range out {
		out[i] = strings.TrimRight(ansi.Strip(out[i]), " ")
	}
	return out
}

// typeText types s into the open search prompt.
func typeText(m *Model, s string) {
	for _, r := range s {
		key(m, press(r, 0, string(r)))
	}
}
