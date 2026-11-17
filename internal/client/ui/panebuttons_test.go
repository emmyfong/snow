package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

func TestPaneButtonsFocusedOnly(t *testing.T) {
	panes := []WindowPane{
		{X: 0, Y: 1, W: 20, H: 3, Kind: "chat"},
		{X: 21, Y: 1, W: 20, H: 3, Kind: "chat", Focused: true},
	}
	buttons := PaneButtons(panes, 41, 4)
	if len(buttons) != 3 {
		t.Fatalf("buttons = %+v, want three", buttons)
	}
	for _, b := range buttons {
		if b.Y != 0 || b.X < 21 || b.X+b.W > 41 {
			t.Errorf("button outside focused pane's top border: %+v", b)
		}
	}
	panes[1].Focused = false
	if got := PaneButtons(panes, 41, 4); len(got) != 0 {
		t.Fatalf("unfocused window has buttons: %+v", got)
	}
}

func TestPaneButtonsModelChatOnly(t *testing.T) {
	for _, kind := range []string{"chat", "terminal", "git", "unknown"} {
		t.Run(kind, func(t *testing.T) {
			buttons := PaneButtons([]WindowPane{{Y: 1, W: 30, H: 2, Focused: true, Kind: kind}}, 30, 3)
			var actions []string
			for _, b := range buttons {
				actions = append(actions, b.Action)
			}
			want := "zoom,close-pane"
			if kind == "chat" {
				want = "model-picker," + want
			}
			if got := strings.Join(actions, ","); got != want {
				t.Fatalf("actions = %q, want %q", got, want)
			}
		})
	}
}

func TestPaneButtonsNarrowKeepsClose(t *testing.T) {
	for _, kind := range []string{"chat", "terminal"} {
		t.Run(kind, func(t *testing.T) {
			for _, width := range []int{1, 2, 3, 4, 5, 8, 12} {
				panes := []WindowPane{{Y: 1, W: width, H: 2, Focused: true, Kind: kind}}
				buttons := PaneButtons(panes, width, 3)
				if len(buttons) != 1 || buttons[0].Action != "close-pane" {
					t.Fatalf("width %d: buttons %+v,want close only", width, buttons)
				}
				b := buttons[0]
				if b.X < 0 || b.X+b.W > width || b.Y != 0 {
					t.Errorf("width %d: button outside border %+v", width, b)
				}
				rows := plain(Window(theme.Default(), WindowOptions{Width: width, Height: 3, Panes: panes}))
				if !strings.Contains(rows[0], theme.Default().Glyphs.Close) {
					t.Errorf("width %d: missing close %q", width, rows[0])
				}
				for _, row := range rows {
					if w := ansi.StringWidth(row); w != width {
						t.Errorf("row width %d,want %d: %q", w, width, row)
					}
				}
			}
		})
	}
}

func TestPaneButtonsDoNotUseBottomOrSideBorder(t *testing.T) {
	panes := []WindowPane{{W: 12, H: 1, Focused: true}, {Y: 2, W: 12, H: 1}}
	if got := PaneButtons(panes, 12, 3); len(got) != 0 {
		t.Fatalf("pane without top border has buttons: %+v", got)
	}
	for _, size := range [][2]int{{0, 1}, {1, 0}, {-1, 2}, {2, -1}} {
		if got := PaneButtons(panes, size[0], size[1]); len(got) != 0 {
			t.Fatalf("empty area %v has buttons: %+v", size, got)
		}
	}
}

func TestPaneButtonsAvoidJunctions(t *testing.T) {
	panes := []WindowPane{
		{Y: 1, W: 10, H: 1},
		{X: 11, Y: 1, W: 20, H: 1},
		{Y: 3, W: 31, H: 2, Focused: true, Kind: "chat", Name: strings.Repeat("n", 50)},
	}
	g := newGrid(WindowOptions{Width: 31, Height: 5, Panes: panes})
	for _, b := range PaneButtons(panes, 31, 5) {
		for x := b.X; x < b.X+b.W; x++ {
			if !g.border(x, b.Y) || g.border(x, b.Y-1) || g.border(x, b.Y+1) {
				t.Fatalf("button overlaps a junction: %+v at %d", b, x)
			}
		}
	}
	row := plain(Window(theme.Default(), WindowOptions{Width: 31, Height: 5, Panes: panes}))[2]
	if got := ansi.Cut(row, 10, 11); got != theme.Default().Glyphs.TeeUp {
		t.Fatalf("junction = %q, want TeeUp: %q", got, row)
	}
}

func TestPaneNameOnBorder(t *testing.T) {
	for _, tt := range []struct {
		name  string
		width int
		want  string
	}{
		{"long-name", 8, "long-n"},
		{"漢字界abc", 7, "漢字"},
		{"\x1b[31mdev\x1b[0m\x00\r\napi\t!", 30, "dev api    !"},
		{"\u0301", 4, ""},
	} {
		panes := []WindowPane{{Y: 1, W: tt.width, H: 2, Name: tt.name, Lines: []string{"body"}}}
		rows := plain(Window(theme.Default(), WindowOptions{Width: tt.width, Height: 3, Panes: panes}))
		if tt.want != "" && !strings.Contains(rows[0], tt.want) {
			t.Errorf("name %q: header %q lacks %q", tt.name, rows[0], tt.want)
		}
		if !strings.HasPrefix(rows[1], "body"[:min(4, tt.width)]) {
			t.Errorf("name changed content row: %q", rows[1])
		}
		for _, row := range rows {
			if w := ansi.StringWidth(row); w != tt.width {
				t.Errorf("name %q: row width %d, want %d: %q", tt.name, w, tt.width, row)
			}
		}
	}
}

func TestWindowDrawsPaneButtonsAtHitCells(t *testing.T) {
	th := theme.Default()
	panes := []WindowPane{{Y: 1, W: 30, H: 2, Focused: true, Kind: "chat", Name: strings.Repeat("漢", 30)}}
	rows := plain(Window(th, WindowOptions{Width: 30, Height: 3, Panes: panes}))
	for _, b := range PaneButtons(panes, 30, 3) {
		glyph := map[string]string{"model-picker": th.Glyphs.Model, "zoom": th.Glyphs.Zoom, "close-pane": th.Glyphs.Close}[b.Action]
		if got, want := ansi.Cut(rows[b.Y], b.X, b.X+b.W), "["+glyph+"]"; got != want {
			t.Errorf("%s hit cells contain %q, want %q", b.Action, got, want)
		}
	}
	if n := strings.Count(strings.Join(rows, "\n"), th.Glyphs.Close); n != 1 {
		t.Fatalf("close count = %d, want 1", n)
	}
}
