package copymode

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/pkg/api"
)

func matchesAt(lines ...int) []api.Match {
	out := make([]api.Match, len(lines))
	for i, l := range lines {
		out[i] = api.Match{Line: l, Col: 2, Len: 3}
	}
	return out
}

func TestFind(t *testing.T) {
	ms := matchesAt(10, 20, 30)
	tests := []struct {
		name     string
		from     pos
		backward bool
		want     int
		wrapped  bool
	}{
		{"forward from before the first", pos{0, 0}, false, 0, false},
		{"forward between matches", pos{15, 0}, false, 1, false},
		{"forward skips the match under the cursor", pos{20, 2}, false, 2, false},
		{"forward from the same line, before the match", pos{20, 1}, false, 1, false},
		{"forward past the last wraps", pos{31, 0}, false, 0, true},
		{"forward from the last match wraps", pos{30, 2}, false, 0, true},
		{"backward from after the last", pos{99, 0}, true, 2, false},
		{"backward between matches", pos{25, 0}, true, 1, false},
		{"backward skips the match under the cursor", pos{20, 2}, true, 0, false},
		{"backward from the first match wraps", pos{10, 2}, true, 2, true},
		{"backward from before the first wraps", pos{0, 0}, true, 2, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, wrapped := find(ms, tt.from, tt.backward)
			if got != tt.want || wrapped != tt.wrapped {
				t.Fatalf("find(%v, backward=%v) = %d, wrapped=%v; want %d, wrapped=%v", tt.from, tt.backward, got, wrapped, tt.want, tt.wrapped)
			}
		})
	}
	if got, _ := find(nil, pos{}, false); got != -1 {
		t.Fatalf("find with no matches = %d, want -1", got)
	}
}

// TestSearchWraps: repeating a search past the last match goes to the first
// and says so.
func TestSearchWraps(t *testing.T) {
	m, srv := open(t, 100)
	key(m, press('g', 0, "g"))
	srv.answer(m)
	key(m, press('/', 0, "/")) // forward, toward newer lines
	typeText(m, "needle")
	key(m, press(tea.KeyEnter, 0, ""))
	if len(srv.searches) != 1 || srv.searches[0].Query != "needle" || srv.searches[0].Pane != 7 {
		t.Fatalf("searches %v, want one for needle on pane 7", srv.searches)
	}
	m.Receive(&api.SearchResult{Pane: 7, Query: "needle", Matches: matchesAt(10, 50, 90)})
	srv.answer(m)
	if m.cur.line != 10 || m.Status() != "" {
		t.Fatalf("first jump: line %d, status %q; want line 10 and no note", m.cur.line, m.Status())
	}
	for _, want := range []int{50, 90} {
		key(m, press('n', 0, "n"))
		srv.answer(m)
		if m.cur.line != want || m.Status() != "" {
			t.Fatalf("n: line %d, status %q; want line %d", m.cur.line, m.Status(), want)
		}
	}
	key(m, press('n', 0, "n"))
	if m.cur.line != 10 || m.Status() != "Search wrapped" {
		t.Fatalf("n past the last: line %d, status %q; want line 10 and a wrap note", m.cur.line, m.Status())
	}
	key(m, press('N', 0, "N"))
	if m.cur.line != 90 || m.Status() != "Search wrapped" {
		t.Fatalf("N before the first: line %d, status %q; want line 90 and a wrap note", m.cur.line, m.Status())
	}
	key(m, press('j', 0, "j"))
	if m.Status() != "" {
		t.Fatalf("the note should clear on the next key, got %q", m.Status())
	}
}

func TestQuestionMarkSearchesTowardOlderLines(t *testing.T) {
	m, srv := open(t, 100)
	key(m, press('?', 0, "?"))
	typeText(m, "x")
	key(m, press(tea.KeyEnter, 0, ""))
	m.Receive(&api.SearchResult{Pane: 7, Query: "x", Matches: matchesAt(10, 50, 99)})
	srv.answer(m)
	if m.cur.line != 50 {
		t.Fatalf("? from the last line landed on %d, want the nearest older match, 50", m.cur.line)
	}
	key(m, press('N', 0, "N"))
	if m.cur.line != 99 {
		t.Fatalf("N after ? went to %d, want 99 (toward newer lines)", m.cur.line)
	}
}

func TestSearchPrompt(t *testing.T) {
	m, srv := open(t, 100)
	key(m, press('?', 0, "?"))
	if got := m.Prompt(); got != "Search up: " {
		t.Fatalf("prompt %q", got)
	}
	typeText(m, "abc")
	key(m, press(tea.KeyBackspace, 0, ""))
	if got := m.Prompt(); got != "Search up: ab" {
		t.Fatalf("prompt %q, want ab after a backspace", got)
	}
	key(m, press('u', tea.ModCtrl, ""))
	typeText(m, "q") // q types a letter in the prompt; it does not leave
	if got := m.Prompt(); got != "Search up: q" {
		t.Fatalf("prompt %q", got)
	}
	if key(m, press(tea.KeyEscape, 0, "")) != Open || m.Prompt() != "" {
		t.Fatal("Esc should close the prompt and keep copy mode open")
	}
	if len(srv.searches) != 0 {
		t.Fatalf("a cancelled prompt sent %v", srv.searches)
	}
	key(m, press('/', 0, "/"))
	if got := m.Prompt(); got != "Search down: " {
		t.Fatalf("prompt %q", got)
	}
	key(m, press(tea.KeyEnter, 0, ""))
	if len(srv.searches) != 0 {
		t.Fatal("an empty query should not be sent")
	}
	key(m, press('q', 0, "q"))
}

// TestSearchAltGr: a character typed with AltGr (Ctrl+Alt with text on
// Windows) goes into the search (#209).
func TestSearchAltGr(t *testing.T) {
	m, _ := open(t, 100)
	key(m, press('/', 0, "/"))
	key(m, press('{', tea.ModCtrl|tea.ModAlt, "{"))
	if got := m.Prompt(); got != "Search down: {" {
		t.Fatalf("prompt %q, want the AltGr character", got)
	}
}

func TestSearchNotes(t *testing.T) {
	m, srv := open(t, 100)
	key(m, press('n', 0, "n"))
	if m.Status() != "No previous search" {
		t.Fatalf("status %q", m.Status())
	}
	key(m, press('?', 0, "?"))
	typeText(m, "zzz")
	key(m, press(tea.KeyEnter, 0, ""))
	m.Receive(&api.SearchResult{Pane: 7, Query: "zzz"})
	srv.answer(m)
	if m.Status() != "No match for zzz" {
		t.Fatalf("status %q", m.Status())
	}
	// A late answer to an old query is dropped.
	key(m, press('?', 0, "?"))
	typeText(m, "new")
	key(m, press(tea.KeyEnter, 0, ""))
	m.Receive(&api.SearchResult{Pane: 7, Query: "zzz", Matches: matchesAt(5)})
	if m.cur.line == 5 {
		t.Fatal("a result for a replaced query moved the cursor")
	}
}

func TestSearchHighlightsVisibleMatches(t *testing.T) {
	m, srv := open(t, 100)
	key(m, press('?', 0, "?"))
	typeText(m, "ine")
	key(m, press(tea.KeyEnter, 0, ""))
	m.Receive(&api.SearchResult{Pane: 7, Query: "ine", Matches: []api.Match{{Line: 2, Col: 1, Len: 3}, {Line: 80, Col: 1, Len: 3}, {Line: 90, Col: 1, Len: 3}}})
	srv.answer(m)
	hs := m.highlights()
	if len(hs) != 2 {
		t.Fatalf("%d highlights, want the two matches in view: %+v", len(hs), hs)
	}
	view := m.View(testTheme)
	if !strings.Contains(view, "\x1b[") {
		t.Fatal("no styles in the view")
	}
	if strings.Count(view, "ine") < 2 {
		t.Fatal("highlighted text is missing")
	}
}
