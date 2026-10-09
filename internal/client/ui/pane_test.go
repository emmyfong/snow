package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestPaneViewSize(t *testing.T) {
	lines := []string{"short", strings.Repeat("x", 200), "\x1b[31mred\x1b[0m", "漢字😀"}
	for _, size := range []struct{ w, h int }{{80, 24}, {120, 40}, {10, 2}, {1, 1}} {
		out := PaneView(lines, size.w, size.h)
		rows := strings.Split(out, "\n")
		if len(rows) != size.h {
			t.Fatalf("%dx%d: %d rows, want %d", size.w, size.h, len(rows), size.h)
		}
		for i, r := range rows {
			if got := ansi.StringWidth(r); got != size.w {
				t.Fatalf("%dx%d: row %d is %d cells wide, want %d: %q", size.w, size.h, i, got, size.w, r)
			}
		}
	}
}

func TestPaneViewZeroSize(t *testing.T) {
	for _, size := range []struct{ w, h int }{{0, 24}, {80, 0}, {-1, -1}} {
		if out := PaneView([]string{"x"}, size.w, size.h); out != "" {
			t.Fatalf("%dx%d: %q, want empty", size.w, size.h, out)
		}
	}
	if out := PaneView(nil, 3, 2); out != "   \n   " {
		t.Fatalf("no lines: %q, want blank 3x2", out)
	}
}
