package copymode

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/pkg/api"
)

// TestCopyModeRebasesWhenLinesDrop: the pane keeps printing with a full
// scrollback, so line numbers shift while pages are in flight. The cursor
// and the selection must stay on the same text, and y must copy it.
func TestCopyModeRebasesWhenLinesDrop(t *testing.T) {
	m, srv := open(t, 300)
	var copied []string
	m.o.Copy = func(text string) tea.Cmd { copied = append(copied, text); return nil }
	for range 50 {
		key(m, press('k', 0, "k"))
	}
	srv.answer(m)
	key(m, press('v', 0, "v")) // on line 249
	for range 100 {
		key(m, press('k', 0, "k")) // to line 149; the pages it needs stay unanswered
	}
	srv.print(10)
	srv.answer(m)
	key(m, press('y', 0, "y"))
	srv.answer(m)
	if len(copied) != 1 {
		t.Fatalf("copied %q, want one copy", copied)
	}
	got := strings.Split(copied[0], "\n")
	if len(got) != 101 || got[0] != "line 149" || got[99] != "line 248" || got[100] != "l" {
		t.Fatalf("copied %d lines from %q to %q, want line 149 to the l of line 249", len(got), got[0], got[len(got)-1])
	}
}

func TestSearchMatchesFollowDroppedLines(t *testing.T) {
	m, srv := open(t, 300)
	key(m, press('?', 0, "?"))
	key(m, press('9', 0, "9"))
	key(m, press(tea.KeyEnter, 0, ""))
	m.Receive(&api.SearchResult{Pane: 7, Query: "9", Matches: []api.Match{{Line: 5, Col: 5, Len: 1}, {Line: 290, Col: 5, Len: 1}}})
	srv.print(10)
	m.Receive(&api.Scrollback{Pane: 7, From: 0, Total: len(srv.lines), Dropped: srv.dropped, Lines: srv.lines[:1]})
	if got := m.search.matches; len(got) != 1 || got[0].Line != 280 {
		t.Fatalf("matches after 10 lines dropped: %v, want only the second, on line 280", got)
	}
}

func TestResizeFetchesTheHistoryAgain(t *testing.T) {
	m, srv := open(t, 300)
	srv.requests = nil
	m.Resize(60, 24)
	if len(srv.requests) == 0 {
		t.Fatal("a resize must fetch the lines again: they were drawn at the old width")
	}
}
