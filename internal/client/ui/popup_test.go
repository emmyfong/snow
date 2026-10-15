package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

func TestPopup(t *testing.T) {
	out := Popup(theme.Default(), PopupOptions{Title: "Split right", Items: []string{"pwsh", "Ubuntu (current)"}, Selected: 1, Width: 24})
	rows := strings.Split(ansi.Strip(out), "\n")
	if len(rows) != 4 {
		t.Fatalf("%d rows, want border, 2 items, border: %q", len(rows), rows)
	}
	if !strings.HasPrefix(rows[0], "┌") || !strings.Contains(rows[0], "Split right") || !strings.HasPrefix(rows[3], "└") {
		t.Fatalf("frame rows %q / %q", rows[0], rows[3])
	}
	if !strings.Contains(rows[2], "> Ubuntu (current)") || strings.Contains(rows[1], ">") {
		t.Fatalf("selection marker wrong: %q / %q", rows[1], rows[2])
	}
	for _, r := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(r); w != 24 {
			t.Fatalf("row width %d, want 24: %q", w, r)
		}
	}
}

func TestPopupIsWidthSafe(t *testing.T) {
	for _, w := range []int{0, 1, 3, 8} {
		out := Popup(theme.Default(), PopupOptions{Title: "a long title here", Items: []string{"漢字漢字漢字漢字"}, Width: w})
		if w < 4 {
			if out != "" {
				t.Errorf("width %d drew %q, want nothing", w, out)
			}
			continue
		}
		for _, r := range strings.Split(out, "\n") {
			if got := ansi.StringWidth(r); got != w {
				t.Errorf("width %d: row %q is %d wide", w, r, got)
			}
		}
	}
}

func TestOverlay(t *testing.T) {
	base := "abcdef\nghijkl\nmnopqr"
	got := Overlay(base, "XY\nZW", 2, 1)
	want := "abcdef\nghXYkl\nmnZWqr"
	if got != want {
		t.Fatalf("Overlay = %q, want %q", got, want)
	}
	if got := Overlay(base, "XYZ", 5, 2); got != "abcdef\nghijkl\nmnopqX" {
		t.Fatalf("clipped overlay = %q", got)
	}
	if got := Overlay(base, "Q", 0, 9); got != base {
		t.Fatalf("overlay below the base changed it: %q", got)
	}
}

func TestPopupUsesThemePointer(t *testing.T) {
	th := theme.Default()
	th.Glyphs.Pointer = "→ "
	got := ansi.Strip(Popup(th, PopupOptions{Items: []string{"a", "b"}, Selected: 1, Width: 12}))
	if !strings.Contains(got, "→ b") || !strings.Contains(got, "  a") {
		t.Fatalf("popup %q ignores the theme's pointer", got)
	}
}
