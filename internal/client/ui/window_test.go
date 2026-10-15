package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

func plain(s string) []string { return strings.Split(ansi.Strip(s), "\n") }

func TestWindowTwoPanes(t *testing.T) {
	out := Window(theme.Default(), WindowOptions{Width: 7, Height: 2, Panes: []WindowPane{
		{X: 0, Y: 0, W: 3, H: 2, Lines: []string{"abc", "de"}},
		{X: 4, Y: 0, W: 3, H: 2, Lines: []string{"xyz"}, Focused: true},
	}})
	want := []string{"abc│xyz", "de │   "}
	if got := plain(out); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWindowJunctions(t *testing.T) {
	// h(1, v(2, 3)): a vertical border with a tee where the horizontal one
	// meets it.
	out := Window(theme.Default(), WindowOptions{Width: 5, Height: 3, Panes: []WindowPane{
		{X: 0, Y: 0, W: 2, H: 3},
		{X: 3, Y: 0, W: 2, H: 1},
		{X: 3, Y: 2, W: 2, H: 1},
	}})
	want := []string{"  │  ", "  ├──", "  │  "}
	if got := plain(out); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestWindowCross(t *testing.T) {
	out := Window(theme.Default(), WindowOptions{Width: 3, Height: 3, Panes: []WindowPane{
		{X: 0, Y: 0, W: 1, H: 1}, {X: 2, Y: 0, W: 1, H: 1},
		{X: 0, Y: 2, W: 1, H: 1}, {X: 2, Y: 2, W: 1, H: 1},
	}})
	if got := plain(out)[1]; got != "─┼─" {
		t.Fatalf("middle row %q, want ─┼─", got)
	}
}

func TestWindowFocusColorsOnlyItsBorders(t *testing.T) {
	th := theme.Default()
	panes := []WindowPane{
		{X: 0, Y: 0, W: 2, H: 3},
		{X: 3, Y: 0, W: 2, H: 1, Focused: true},
		{X: 3, Y: 2, W: 2, H: 1},
	}
	focused := Window(th, WindowOptions{Width: 5, Height: 3, Panes: panes})
	panes[1].Focused, panes[0].Focused = false, true
	other := Window(th, WindowOptions{Width: 5, Height: 3, Panes: panes})
	if focused == other {
		t.Fatal("moving the focus did not change the border colors")
	}
}

func TestWindowIsWidthSafe(t *testing.T) {
	tests := []WindowOptions{
		{Width: 0, Height: 0},
		{Width: 4, Height: 2},
		{Width: 3, Height: 1, Panes: []WindowPane{{X: 0, Y: 0, W: 10, H: 5, Lines: []string{"漢字漢字漢字"}}}},
		// A client larger than the session: blank cells around the panes.
		{Width: 10, Height: 3, Panes: []WindowPane{{X: 0, Y: 0, W: 4, H: 1, Lines: []string{"ab"}}}},
	}
	for _, o := range tests {
		out := Window(theme.Default(), o)
		rows := strings.Split(out, "\n")
		if o.Height == 0 {
			if out != "" {
				t.Errorf("empty window drew %q", out)
			}
			continue
		}
		if len(rows) != o.Height {
			t.Errorf("%+v: %d rows, want %d", o, len(rows), o.Height)
		}
		for _, r := range rows {
			if w := ansi.StringWidth(r); w != o.Width {
				t.Errorf("%+v: row width %d, want %d: %q", o, w, o.Width, r)
			}
		}
	}
}
