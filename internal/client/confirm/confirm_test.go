package confirm

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

func newModel() *Model {
	return New("Close pane", "Close this pane (Ubuntu /src, 2 of 3)?", "Close")
}

func TestKeys(t *testing.T) {
	tests := []struct {
		name string
		keys []tea.Key
		want Result
	}{
		{"y", []tea.Key{{Code: 'y', Text: "y"}}, Yes},
		{"Y", []tea.Key{{Code: 'Y', Text: "Y"}}, Yes},
		{"n", []tea.Key{{Code: 'n', Text: "n"}}, No},
		{"escape", []tea.Key{{Code: tea.KeyEscape}}, No},
		{"enter cancels by default", []tea.Key{{Code: tea.KeyEnter}}, No},
		{"tab then enter confirms", []tea.Key{{Code: tea.KeyTab}, {Code: tea.KeyEnter}}, Yes},
		{"left then enter confirms", []tea.Key{{Code: tea.KeyLeft}, {Code: tea.KeyEnter}}, Yes},
		{"left right enter cancels", []tea.Key{{Code: tea.KeyLeft}, {Code: tea.KeyRight}, {Code: tea.KeyEnter}}, No},
		{"other keys do nothing", []tea.Key{{Code: 'q', Text: "q"}, {Code: tea.KeyDown}}, Open},
	}
	for _, tt := range tests {
		m := newModel()
		got := Open
		for _, k := range tt.keys {
			got = m.Key(k)
		}
		if got != tt.want {
			t.Errorf("%s: %v, want %v", tt.name, got, tt.want)
		}
	}
}

// button finds a button label in the drawn box and returns a screen cell
// inside it.
func button(t *testing.T, box string, bx, by int, label string) (int, int) {
	t.Helper()
	for i, row := range strings.Split(ansi.Strip(box), "\n") {
		if x := strings.Index(row, label); x >= 0 {
			return bx + ansi.StringWidth(row[:x]) + 1, by + i
		}
	}
	t.Fatalf("no %q in\n%s", label, ansi.Strip(box))
	return 0, 0
}

func TestClickButtons(t *testing.T) {
	th := theme.Default()
	m := newModel()
	box, bx, by := m.View(th, 80, 23)
	cx, cy := button(t, box, bx, by, "[ Close ]")
	if got := m.Click(th, 80, 23, cx, cy); got != Yes {
		t.Errorf("click on Close = %v, want Yes", got)
	}
	cx, cy = button(t, box, bx, by, "[ Cancel ]")
	if got := m.Click(th, 80, 23, cx, cy); got != No {
		t.Errorf("click on Cancel = %v, want No", got)
	}
	if got := m.Click(th, 80, 23, bx+2, by+1); got != Open {
		t.Errorf("click on the question = %v, want Open", got)
	}
	if got := m.Click(th, 80, 23, 0, 0); got != No {
		t.Errorf("click outside = %v, want No", got)
	}
}

func TestViewIsCenteredAndWidthSafe(t *testing.T) {
	th := theme.Default()
	box, x, y := newModel().View(th, 80, 23)
	w, h := 0, strings.Count(box, "\n")+1
	for _, r := range strings.Split(box, "\n") {
		w = max(w, ansi.StringWidth(r))
	}
	if x != (80-w)/2 || y != (23-h)/2 {
		t.Errorf("at %d,%d, want centered", x, y)
	}
	text := ansi.Strip(box)
	for _, want := range []string{"Close pane", "Close this pane", "[ Close ]", "[ Cancel ]"} {
		if !strings.Contains(text, want) {
			t.Errorf("popup lacks %q:\n%s", want, text)
		}
	}
	for cols := 0; cols <= 60; cols += 5 {
		for rows := 0; rows <= 14; rows += 2 {
			box, bx, by := newModel().View(th, cols, rows)
			if box == "" {
				continue
			}
			bh := strings.Count(box, "\n") + 1
			if bx < 0 || by < 0 || by+bh > rows {
				t.Fatalf("screen %dx%d: box at %d,%d is %d rows", cols, rows, bx, by, bh)
			}
			for _, r := range strings.Split(box, "\n") {
				if bx+ansi.StringWidth(r) > cols {
					t.Fatalf("screen %dx%d: row %q leaves the screen", cols, rows, r)
				}
			}
		}
	}
}
