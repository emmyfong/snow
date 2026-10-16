package copymode

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// TestSelectionText: the text of a selection that runs over several lines,
// including a wide character and lines with trailing spaces.
func TestSelectionText(t *testing.T) {
	lines := []string{
		"first line of a long command that the terminal wrapped",
		"and its second half   ",
		"",
		"漢字 wide text",
		"last line",
	}
	get := func(i int) string { return strings.TrimRight(lines[i], " ") }
	tests := []struct {
		name     string
		from, to pos
		want     string
	}{
		{"one line, a few cells", pos{0, 6}, pos{0, 9}, "line"},
		{"one cell", pos{0, 0}, pos{0, 0}, "f"},
		{"across wrapped lines", pos{0, 34}, pos{1, 2}, "the terminal wrapped\nand"},
		{"whole lines keep blank ones", pos{1, 0}, pos{3, 3}, "and its second half\n\n漢字"},
		{"end past the text", pos{1, 8}, pos{1, 80}, "second half"},
		{"wide character kept whole", pos{3, 0}, pos{3, 1}, "漢"},
		{"ending on a wide character's second cell keeps it", pos{3, 0}, pos{3, 3}, "漢字"},
		{"starting on its second cell drops it", pos{3, 1}, pos{3, 4}, "字"},
		{"to the end of the history", pos{3, 5}, pos{4, 8}, "wide text\nlast line"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := selectionText(get, tt.from, tt.to); got != tt.want {
				t.Fatalf("selectionText(%v, %v) = %q, want %q", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

// copied collects what copy mode puts on the clipboard.
type copied struct{ texts []string }

type copyDone struct{}

func (c *copied) copy(text string) tea.Cmd {
	c.texts = append(c.texts, text)
	return func() tea.Msg { return copyDone{} }
}

func openWithCopy(t *testing.T, n int) (*Model, *fakeServer, *copied) {
	t.Helper()
	srv := &fakeServer{lines: numbered(n)}
	c := &copied{}
	screen := srv.lines[max(n-24, 0):]
	m := New(Options{Pane: 7, Cols: 80, Rows: 24, Screen: screen, CursorY: len(screen) - 1, Send: srv.send, Copy: c.copy})
	srv.answer(m)
	return m, srv, c
}

func TestSelectAndYank(t *testing.T) {
	m, _, c := openWithCopy(t, 100)
	key(m, press('k', 0, "k"))
	key(m, press('k', 0, "k"))
	key(m, press('v', 0, "v"))
	key(m, press('j', 0, "j"))
	key(m, press('l', 0, "l"))
	key(m, press('l', 0, "l"))
	res, cmd := m.Key(press('y', 0, "y"))
	if res != Exited || cmd == nil {
		t.Fatalf("y returned %v, cmd=%v; want Exited and the copy command", res, cmd)
	}
	if want := "line 97\nlin"; len(c.texts) != 1 || c.texts[0] != want {
		t.Fatalf("copied %q, want %q", c.texts, want)
	}
	if m.Status() != "Copied 11 characters" {
		t.Fatalf("status %q", m.Status())
	}
}

func TestSelectionBackwards(t *testing.T) {
	m, _, c := openWithCopy(t, 100)
	key(m, press('k', 0, "k"))
	key(m, press('v', 0, "v"))
	key(m, press('k', 0, "k"))
	key(m, press(tea.KeyEnter, 0, ""))
	if want := "line 97\nl"; len(c.texts) != 1 || c.texts[0] != want {
		t.Fatalf("copied %q, want %q: Enter copies, and a selection made upward reads in order", c.texts, want)
	}
}

func TestEscapeClearsSelectionFirst(t *testing.T) {
	m, _, c := openWithCopy(t, 100)
	key(m, press('v', 0, "v"))
	if key(m, press(tea.KeyEscape, 0, "")) != Open || m.selecting {
		t.Fatal("Esc should clear the selection and keep copy mode open")
	}
	if key(m, press(tea.KeyEscape, 0, "")) != Exited {
		t.Fatal("a second Esc should leave copy mode")
	}
	if len(c.texts) != 0 {
		t.Fatalf("leaving copied %q", c.texts)
	}
}

func TestYankWithoutSelection(t *testing.T) {
	m, _, c := openWithCopy(t, 100)
	if key(m, press('y', 0, "y")) != Open || m.Status() != "Nothing selected" || len(c.texts) != 0 {
		t.Fatalf("y with no selection: status %q, copied %q", m.Status(), c.texts)
	}
	key(m, press('v', 0, "v"))
	key(m, press('v', 0, "v"))
	key(m, press('y', 0, "y"))
	if m.Status() != "Nothing selected" {
		t.Fatal("v twice cancels the selection")
	}
}

// A selection can reach lines that have not been fetched; the copy waits for
// them.
func TestYankWaitsForLines(t *testing.T) {
	m, srv, c := openWithCopy(t, 2000)
	key(m, press('v', 0, "v"))
	key(m, press('g', 0, "g"))
	srv.answer(m)
	if res, _ := m.Key(press('y', 0, "y")); res != Open || len(c.texts) != 0 {
		t.Fatal("y should wait for the lines between the cursor and the top")
	}
	if len(srv.pending) == 0 {
		t.Fatal("y asked for no lines")
	}
	var res Result
	var cmd tea.Cmd
	for len(srv.pending) > 0 {
		r := srv.pending[0]
		srv.pending = srv.pending[1:]
		res, cmd = m.Receive(&api.Scrollback{Pane: 7, From: r.From, Total: 2000, Lines: srv.lines[r.From : r.From+r.Count]})
	}
	if res != Exited || cmd == nil || len(c.texts) != 1 {
		t.Fatalf("after the last page: %v, cmd=%v, copied %d times", res, cmd, len(c.texts))
	}
	if got := strings.Count(c.texts[0], "\n"); got != 1999 {
		t.Fatalf("copied %d line breaks, want 1999 (all 2,000 lines)", got)
	}
}

func TestSelectionIsHighlighted(t *testing.T) {
	m, _, _ := openWithCopy(t, 100)
	key(m, press('k', 0, "k"))
	key(m, press('v', 0, "v"))
	key(m, press('j', 0, "j"))
	key(m, press('l', 0, "l"))
	hs := m.selection()
	if len(hs) != 2 || hs[0].Kind != ui.HighlightSelected {
		t.Fatalf("highlights %+v, want two selected rows", hs)
	}
	// The cursor is at col 1 of line 99: the last row covers cells 0 and 1;
	// the first row runs from col 0 to the end of the text.
	if hs[0].Col != 0 || hs[0].Len != 7 || hs[1].Col != 0 || hs[1].Len != 2 {
		t.Fatalf("highlights %+v, want rows of 7 and 2 cells", hs)
	}
}

func TestMouseDragCopies(t *testing.T) {
	m, srv, c := openWithCopy(t, 100)
	// The view shows lines 76 to 99 in rows 0 to 23.
	m.Mouse(tea.MouseClickMsg{X: 1, Y: 2, Button: tea.MouseLeft}, 1, 2)
	if m.selecting {
		t.Fatal("a click alone selects nothing")
	}
	m.Mouse(tea.MouseMotionMsg{X: 4, Y: 3, Button: tea.MouseLeft}, 4, 3)
	if !m.selecting {
		t.Fatal("motion with the button down should select")
	}
	res, cmd := m.Mouse(tea.MouseReleaseMsg{X: 4, Y: 3, Button: tea.MouseLeft}, 4, 3)
	srv.answer(m)
	if res != Exited || cmd == nil {
		t.Fatalf("release returned %v, cmd=%v; want Exited and the copy command", res, cmd)
	}
	if want := "ine 78\nline"; len(c.texts) != 1 || c.texts[0] != want {
		t.Fatalf("copied %q, want %q", c.texts, want)
	}
}

func TestMouseClickWithoutDragDoesNotCopy(t *testing.T) {
	m, _, c := openWithCopy(t, 100)
	m.Mouse(tea.MouseClickMsg{X: 2, Y: 5, Button: tea.MouseLeft}, 2, 5)
	res, cmd := m.Mouse(tea.MouseReleaseMsg{X: 2, Y: 5, Button: tea.MouseLeft}, 2, 5)
	if res != Open || cmd != nil || len(c.texts) != 0 {
		t.Fatalf("a click copied: %v %v %q", res, cmd, c.texts)
	}
	if m.cur.line != 81 || m.cur.col != 2 {
		t.Fatalf("cursor at %v, want the clicked cell, line 81 col 2", m.cur)
	}
}

func TestMouseOutsideThePaneIsIgnored(t *testing.T) {
	m, _, _ := openWithCopy(t, 100)
	before := m.cur
	m.Mouse(tea.MouseClickMsg{X: 90, Y: 5, Button: tea.MouseLeft}, 90, 5)
	m.Mouse(tea.MouseClickMsg{X: 1, Y: -1, Button: tea.MouseLeft}, 1, -1)
	m.Mouse(tea.MouseClickMsg{X: 1, Y: 1, Button: tea.MouseRight}, 1, 1)
	if m.cur != before || m.dragging {
		t.Fatalf("cursor moved to %v or a drag started", m.cur)
	}
}

func TestMouseWheelScrolls(t *testing.T) {
	m, srv, _ := openWithCopy(t, 500)
	top := m.top
	m.Mouse(tea.MouseWheelMsg{Button: tea.MouseWheelUp}, 5, 5)
	srv.answer(m)
	if m.top != top-3 {
		t.Fatalf("wheel up: top %d, want %d", m.top, top-3)
	}
	if m.cur.line < m.top || m.cur.line >= m.top+24 {
		t.Fatalf("cursor line %d left the view from %d", m.cur.line, m.top)
	}
	m.Mouse(tea.MouseWheelMsg{Button: tea.MouseWheelDown}, 5, 5)
	if m.top != top {
		t.Fatalf("wheel down: top %d, want %d", m.top, top)
	}
	m.Mouse(tea.MouseWheelMsg{Button: tea.MouseWheelDown}, 5, 5)
	if m.top != top {
		t.Fatal("the wheel scrolled past the end")
	}
}

func TestMouseDragAboveThePaneScrolls(t *testing.T) {
	m, srv, _ := openWithCopy(t, 500)
	top := m.top
	m.Mouse(tea.MouseClickMsg{X: 1, Y: 5, Button: tea.MouseLeft}, 1, 5)
	m.Mouse(tea.MouseMotionMsg{X: 1, Y: -1, Button: tea.MouseLeft}, 1, -1)
	srv.answer(m)
	if m.top != top-1 || !m.selecting {
		t.Fatalf("top %d, selecting %v: a drag above the pane should scroll one line and select", m.top, m.selecting)
	}
}

// TestKeyCancelsAPendingCopy: y waits for missing lines; a key pressed
// before they arrive cancels the copy, so it never copies a changed
// selection later.
func TestKeyCancelsAPendingCopy(t *testing.T) {
	m, srv := open(t, 300)
	var copied []string
	m.o.Copy = func(text string) tea.Cmd { copied = append(copied, text); return nil }
	key(m, press('v', 0, "v"))
	for range 150 {
		key(m, press('k', 0, "k"))
	}
	key(m, press('y', 0, "y")) // lines are missing: the copy waits
	key(m, press('j', 0, "j"))
	srv.answer(m)
	if len(copied) != 0 {
		t.Fatalf("copied %q after the user moved on", copied)
	}
}

// TestWideCharacterIsNeverSplit: the cursor and a selection never start on
// the second cell of a wide character, so the copy keeps it whole.
func TestWideCharacterIsNeverSplit(t *testing.T) {
	lines := []string{"ab漢字cd", "next"}
	var copied []string
	m := New(Options{Pane: 1, Cols: 20, Rows: 2, Screen: lines, Send: func(api.Message) {},
		Copy: func(text string) tea.Cmd { copied = append(copied, text); return nil }})
	m.Receive(&api.Scrollback{Pane: 1, From: 0, Total: 2, Lines: lines})
	m.Mouse(tea.MouseClickMsg{X: 3, Y: 0, Button: tea.MouseLeft}, 3, 0) // the right half of 漢
	if m.cur.col != 2 {
		t.Fatalf("press on the right half of a wide character: column %d, want 2", m.cur.col)
	}
	m.Mouse(tea.MouseReleaseMsg{X: 5, Y: 0, Button: tea.MouseLeft}, 5, 0) // the right half of 字
	if len(copied) != 1 || copied[0] != "漢字" {
		t.Fatalf("copied %q, want both wide characters whole", copied)
	}
	m2 := New(Options{Pane: 1, Cols: 20, Rows: 2, Screen: lines, Send: func(api.Message) {}})
	m2.Receive(&api.Scrollback{Pane: 1, From: 0, Total: 2, Lines: lines})
	key(m2, press('k', 0, "k"))
	key(m2, press('0', 0, "0"))
	var cols []int
	for range 5 {
		key(m2, press('l', 0, "l"))
		cols = append(cols, m2.cur.col)
	}
	if want := []int{1, 2, 4, 6, 7}; !slices.Equal(cols, want) {
		t.Fatalf("l steps through columns %v, want %v", cols, want)
	}
}
