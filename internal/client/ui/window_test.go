package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/theme"
)

func plain(s string) []string { return strings.Split(ansi.Strip(s), "\n") }

func TestWindowTwoPanes(t *testing.T) {
	out := Window(theme.Default(), WindowOptions{Width: 7, Height: 2, Panes: []WindowPane{
		{X: 0, Y: 0, W: 3, H: 2, Lines: []string{"abc", "de"}},
		{X: 4, Y: 0, W: 3, H: 2, Lines: []string{"xyz"}, Focused: true},
	}})
	want := []string{"abc×xyz", "de │   "}
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

// TestWindowMarksDrivenPane: a pane an agent types into has its borders in
// the accent color, in every built-in theme.
func TestWindowMarksDrivenPane(t *testing.T) {
	for _, th := range theme.Builtin() {
		t.Run(strings.ReplaceAll(th.Name, " ", "_"), func(t *testing.T) {
			// Two rows, so the focused pane's close mark takes one border
			// cell and an accent-colored border cell remains.
			out := Window(th, WindowOptions{Width: 7, Height: 2, Panes: []WindowPane{
				{X: 0, Y: 0, W: 3, H: 2},
				{X: 4, Y: 0, W: 3, H: 2, Focused: true, Driven: true},
			}})
			golden.RequireEqual(t, out)
			accent := lipgloss.NewStyle().Foreground(th.Accent).Render("│")
			if !strings.Contains(out, accent) {
				t.Errorf("%s: driven pane's border %q lacks the accent color", th.Name, out)
			}
		})
	}
}

// TestWindowDrivenBorderWins: the accent color wins over the focus color
// when an agent types into the focused pane.
func TestWindowDrivenBorderWins(t *testing.T) {
	th := theme.Default()
	th.Accent = lipgloss.Color("#ff0000")
	th.FocusBorder = lipgloss.Color("#00ff00")
	out := Window(th, WindowOptions{Width: 7, Height: 2, Panes: []WindowPane{
		{X: 0, Y: 0, W: 3, H: 2},
		{X: 4, Y: 0, W: 3, H: 2, Focused: true, Driven: true},
	}})
	if accent := lipgloss.NewStyle().Foreground(th.Accent).Render("│"); !strings.Contains(out, accent) {
		t.Errorf("border %q lacks the accent color", out)
	}
	if focus := lipgloss.NewStyle().Foreground(th.FocusBorder).Render("│"); strings.Contains(out, focus) {
		t.Errorf("border %q kept the focus color", out)
	}
}

// TestCloseCell: the mark sits at the focused pane's top-right corner when a
// border is there, else at the top of its right or left border. The only pane
// in a window has no border and so no mark.
func TestCloseCell(t *testing.T) {
	sideBySide := func(focus int) []WindowPane {
		panes := []WindowPane{{X: 0, Y: 0, W: 3, H: 2}, {X: 4, Y: 0, W: 3, H: 2}}
		panes[focus].Focused = true
		return panes
	}
	stacked := []WindowPane{{X: 0, Y: 0, W: 3, H: 1}, {X: 0, Y: 2, W: 3, H: 1, Focused: true}}
	three := func(focus int) []WindowPane {
		panes := []WindowPane{{X: 0, Y: 0, W: 2, H: 3}, {X: 3, Y: 0, W: 2, H: 1}, {X: 3, Y: 2, W: 2, H: 1}}
		panes[focus].Focused = true
		return panes
	}
	tests := []struct {
		name  string
		panes []WindowPane
		w, h  int
		x, y  int
		ok    bool
	}{
		{"left pane of two", sideBySide(0), 7, 2, 3, 0, true},
		{"right pane of two", sideBySide(1), 7, 2, 3, 0, true},
		{"bottom of two stacked", stacked, 3, 3, 2, 1, true},
		{"left of three", three(0), 5, 3, 2, 0, true},
		{"top right of three", three(1), 5, 3, 2, 0, true},
		{"bottom right of three", three(2), 5, 3, 4, 1, true},
		{"only pane", []WindowPane{{X: 0, Y: 0, W: 5, H: 3, Focused: true}}, 5, 3, 0, 0, false},
		{"no focus", []WindowPane{{X: 0, Y: 0, W: 3, H: 2}, {X: 4, Y: 0, W: 3, H: 2}}, 7, 2, 0, 0, false},
	}
	for _, tt := range tests {
		x, y, ok := CloseCell(tt.panes, tt.w, tt.h)
		if ok != tt.ok || (ok && (x != tt.x || y != tt.y)) {
			t.Errorf("%s: got (%d, %d, %v), want (%d, %d, %v)", tt.name, x, y, ok, tt.x, tt.y, tt.ok)
		}
	}
}

// TestWindowDrawsCloseMark: the mark is one × at the cell CloseCell names.
func TestWindowDrawsCloseMark(t *testing.T) {
	panes := []WindowPane{{X: 0, Y: 0, W: 2, H: 3}, {X: 3, Y: 0, W: 2, H: 1, Focused: true}, {X: 3, Y: 2, W: 2, H: 1}}
	out := Window(theme.Default(), WindowOptions{Width: 5, Height: 3, Panes: panes})
	rows := plain(out)
	if rows[0] != "  ×  " {
		t.Fatalf("top row %q, want the mark at its corner", rows[0])
	}
	if n := strings.Count(ansi.Strip(out), "×"); n != 1 {
		t.Fatalf("%d marks on screen, want 1", n)
	}
	if x, y, ok := CloseCell(panes, 5, 3); !ok || x != 2 || y != 0 {
		t.Fatalf("CloseCell = (%d, %d, %v), want (2, 0, true)", x, y, ok)
	}
}
