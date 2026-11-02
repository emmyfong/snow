package term

import (
	"strings"
	"testing"
)

// TestSnapshotPlainScreen: the screen comes back as plain rows, with the
// cursor and size, and no styles.
func TestSnapshotPlainScreen(t *testing.T) {
	p, f := newTestPane(t)
	f.shellWrites("\x1b[31mhello\x1b[0m\r\nnext")
	eventually(t, "next", func() bool { return strings.Contains(p.Text(), "next") })

	snap := p.Snapshot(0)
	if snap.Cols != 80 || snap.Rows != 24 {
		t.Errorf("size %dx%d, want 80x24", snap.Cols, snap.Rows)
	}
	if !snap.CursorVisible || snap.CursorX != 4 || snap.CursorY != 1 {
		t.Errorf("cursor (%d,%d) visible %v, want (4,1) visible", snap.CursorX, snap.CursorY, snap.CursorVisible)
	}
	text := strings.Join(snap.Screen, "\n")
	if !strings.Contains(text, "hello") || !strings.Contains(text, "next") {
		t.Fatalf("screen %q, want both lines", text)
	}
	if strings.Contains(text, "\x1b") {
		t.Fatalf("screen keeps styles: %q", text)
	}
	if len(snap.Scrollback) != 0 || snap.Omitted != 0 {
		t.Errorf("scrollback %q, omitted %d; want none", snap.Scrollback, snap.Omitted)
	}
}

// TestSnapshotScrollbackTail: the newest scrollback lines come back oldest
// first, plain, with the older ones counted.
func TestSnapshotScrollbackTail(t *testing.T) {
	p, f := newTestPane(t)
	f.shellWrites("\x1b[32mgreen\x1b[0m\r\n")
	writeLines(t, p, f, 40)
	kept := p.ScrollbackLen()
	if kept < 10 {
		t.Fatalf("scrollback holds %d lines, want at least 10", kept)
	}

	snap := p.Snapshot(5)
	if len(snap.Scrollback) != 5 || snap.Omitted != kept-5 {
		t.Fatalf("snapshot has %d lines, omitted %d; want 5 lines, %d omitted", len(snap.Scrollback), snap.Omitted, kept-5)
	}
	first := lineNumber(t, snap.Scrollback[0])
	for i := 1; i < len(snap.Scrollback); i++ {
		if got := lineNumber(t, snap.Scrollback[i]); got != first+i {
			t.Fatalf("line %d is %d, want %d: oldest first", i, got, first+i)
		}
	}
	if last := lineNumber(t, snap.Scrollback[len(snap.Scrollback)-1]); last != first+4 {
		t.Fatalf("last line is %d, want %d", last, first+4)
	}
	for _, line := range snap.Scrollback {
		if strings.Contains(line, "\x1b") {
			t.Fatalf("scrollback keeps styles: %q", line)
		}
	}
}

// TestSnapshotCopyIsStable: a snapshot taken before more output keeps its
// text, because it copied the lines.
func TestSnapshotCopyIsStable(t *testing.T) {
	p, f := newTestPane(t)
	f.shellWrites("first\r\n")
	eventually(t, "first", func() bool { return strings.Contains(p.Text(), "first") })
	snap := p.Snapshot(0)

	writeLines(t, p, f, 40)
	if !strings.Contains(strings.Join(snap.Screen, "\n"), "first") || strings.Contains(strings.Join(snap.Screen, "\n"), "line 39") {
		t.Fatalf("an old snapshot changed: %q", snap.Screen)
	}
}
