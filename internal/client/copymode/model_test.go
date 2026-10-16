package copymode

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

var testTheme = theme.Default()

// firstRow returns the view's first row without the position marker.
func firstRow(m *Model) string {
	row := rows(m)[0]
	if i := strings.Index(row, "["); i >= 0 {
		row = row[:i]
	}
	return strings.TrimSpace(row)
}

func TestCopyModePaging(t *testing.T) {
	const total = 5000
	m, srv := open(t, total)
	if got := rows(m)[1]; got != fmt.Sprintf("line %d", total-23) {
		t.Fatalf("opens at the bottom: second row %q", got)
	}

	// Page up through the whole buffer, one screen at a time.
	want := total - 24
	pages := 0
	for want > 0 {
		key(m, press(tea.KeyPgUp, 0, ""))
		srv.answer(m)
		want = max(want-24, 0)
		pages++
		if got := firstRow(m); got != fmt.Sprintf("line %d", want) {
			t.Fatalf("page %d up: first row %q, want line %d", pages, got, want)
		}
	}
	if pages != 208 {
		t.Fatalf("%d pages up, want 208 for 5,000 lines of 24", pages)
	}
	if last := rows(m)[23]; last != "line 23" {
		t.Fatalf("top page ends with %q, want line 23", last)
	}

	// And back down to the end.
	for range pages {
		key(m, press(tea.KeyPgDown, 0, ""))
		srv.answer(m)
	}
	if got := rows(m)[23]; got != fmt.Sprintf("line %d", total-1) {
		t.Fatalf("bottom page ends with %q, want line %d", got, total-1)
	}
	if !strings.Contains(m.View(testTheme), "[5000/5000]") {
		t.Fatal("the indicator should read [5000/5000] at the last line")
	}
}

func TestCopyModeFetchesInBoundedPages(t *testing.T) {
	m, srv := open(t, 5000)
	for range 60 {
		key(m, press(tea.KeyPgUp, 0, ""))
		srv.answer(m)
	}
	seen := map[int]int{}
	for _, r := range srv.requests[1:] { // the first request only asks for the size
		if r.Count < 1 || r.Count > api.MaxScrollbackLines || r.From < 0 || r.From+r.Count > 5000 {
			t.Fatalf("request %+v is outside the allowed pages or the history", r)
		}
		for i := r.From; i < r.From+r.Count; i++ {
			seen[i]++
			if seen[i] > 1 {
				t.Fatalf("line %d requested twice", i)
			}
		}
	}
	if len(seen) > 70*24 {
		t.Fatalf("%d lines fetched after 60 pages: lines far from the view should not be fetched", len(seen))
	}
}

func TestKeys(t *testing.T) {
	tests := []struct {
		name     string
		keys     []tea.Key
		wantLine int // the cursor's line, from 0
		wantCol  int
	}{
		{"up", []tea.Key{press(tea.KeyUp, 0, "")}, 98, 0},
		{"k", []tea.Key{press('k', 0, "k")}, 98, 0},
		{"down at the end stays", []tea.Key{press(tea.KeyDown, 0, "")}, 99, 0},
		{"g goes to the top", []tea.Key{press('g', 0, "g")}, 0, 0},
		{"G returns to the end", []tea.Key{press('g', 0, "g"), press('G', 0, "G")}, 99, 0},
		{"page up", []tea.Key{press(tea.KeyPgUp, 0, "")}, 75, 0},
		{"ctrl-b is a page up", []tea.Key{press('b', tea.ModCtrl, "")}, 75, 0},
		{"ctrl-u is half a page", []tea.Key{press('u', tea.ModCtrl, "")}, 87, 0},
		{"right moves one cell", []tea.Key{press(tea.KeyRight, 0, "")}, 99, 1},
		{"l then h", []tea.Key{press('l', 0, "l"), press('l', 0, "l"), press('h', 0, "h")}, 99, 1},
		{"$ goes to the last cell", []tea.Key{press('$', 0, "$")}, 99, 6},
		{"0 goes back", []tea.Key{press('$', 0, "$"), press('0', 0, "0")}, 99, 0},
		{"left stops at the start", []tea.Key{press(tea.KeyLeft, 0, "")}, 99, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, srv := open(t, 100)
			for _, k := range tt.keys {
				if key(m, k) != Open {
					t.Fatal("copy mode closed")
				}
				srv.answer(m)
			}
			if m.cur.line != tt.wantLine || m.cur.col != tt.wantCol {
				t.Fatalf("cursor at line %d col %d, want line %d col %d", m.cur.line, m.cur.col, tt.wantLine, tt.wantCol)
			}
			if m.cur.line < m.top || m.cur.line >= m.top+24 {
				t.Fatalf("cursor line %d is outside the view starting at %d", m.cur.line, m.top)
			}
		})
	}
}

func TestCursorKeepsItsColumnAcrossShortLines(t *testing.T) {
	srv := &fakeServer{lines: []string{"a long first line", "x", "another long line"}}
	m := New(Options{Pane: 1, Cols: 80, Rows: 3, Screen: srv.lines, CursorY: 0, CursorX: 8, Send: srv.send})
	srv.answer(m)
	key(m, press(tea.KeyDown, 0, ""))
	if m.cur.col != 0 {
		t.Fatalf("on the one-cell line the cursor is at col %d, want 0", m.cur.col)
	}
	key(m, press(tea.KeyDown, 0, ""))
	if m.cur.col != 8 {
		t.Fatalf("on the next long line the cursor is back at col %d, want 8", m.cur.col)
	}
}

func TestExit(t *testing.T) {
	for _, k := range []tea.Key{press('q', 0, "q"), press(tea.KeyEscape, 0, "")} {
		m, _ := open(t, 100)
		if key(m, k) != Exited {
			t.Errorf("%v did not leave copy mode", k)
		}
	}
	// Before the history arrives, copy mode can still be left.
	m := New(Options{Pane: 1, Cols: 80, Rows: 24, Send: func(api.Message) {}})
	if key(m, press('q', 0, "q")) != Exited {
		t.Error("q did not leave copy mode while loading")
	}
}

func TestLoadingShowsTheScreen(t *testing.T) {
	m := New(Options{Pane: 1, Cols: 20, Rows: 2, Screen: []string{"one", "two"}, Send: func(api.Message) {}})
	if got := rows(m); got[0] != "one            [...]" || got[1] != "two" {
		t.Fatalf("rows %q: want the screen as it was, with a loading marker", got)
	}
	if _, _, ok := m.Cursor(); ok {
		t.Fatal("no cursor while loading")
	}
	key(m, press(tea.KeyDown, 0, ""))
	m.Receive(&api.Scrollback{Pane: 1, From: 0, Total: 2, Lines: []string{"one"}})
	if m.cur.line != 0 {
		t.Fatalf("a key while loading moved the cursor to line %d, want 0 (the pane cursor)", m.cur.line)
	}
}

func TestReceiveIgnoresOtherPanes(t *testing.T) {
	m, _ := open(t, 100)
	before := m.View(testTheme)
	m.Receive(&api.Scrollback{Pane: 99, From: 0, Total: 5, Lines: []string{"x"}})
	if m.View(testTheme) != before || m.total != 100 {
		t.Fatal("a page for another pane changed copy mode")
	}
}

func TestResizeKeepsCursorInView(t *testing.T) {
	m, srv := open(t, 100)
	m.Resize(40, 10)
	srv.answer(m)
	if m.cur.line < m.top || m.cur.line >= m.top+10 {
		t.Fatalf("cursor line %d is outside the view of 10 rows from %d", m.cur.line, m.top)
	}
	if got := rows(m); len(got) != 10 {
		t.Fatalf("%d rows, want 10", len(got))
	}
}
