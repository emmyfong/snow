package menu

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

func TestPaneMenuScroll(t *testing.T) {
	items := make([]Item, 30)
	for i := range items {
		items[i] = Item{Label: fmt.Sprintf("Action %02d", i), Shortcut: "x", Action: fmt.Sprint(i)}
	}
	th := theme.Default()
	if box, _, _ := New(items, 0, 0).View(th, 80, 22); box != "" {
		t.Fatal("default menu behavior changed")
	}
	m := NewPane(items, 0, 0)
	box, x, y := m.View(th, 80, 22)
	if box == "" || strings.Count(box, "\n")+1 > 22 || !strings.Contains(ansi.Strip(box), "Scroll down") {
		t.Fatalf("pane menu does not scroll:\n%s", box)
	}
	// The mouse can page to every item without mouse motion or key input.
	for i := 0; i < 3 && !strings.Contains(ansi.Strip(box), "Action 29"); i++ {
		h := strings.Count(box, "\n") + 1
		if r := m.Click(th, 80, 22, x+2, y+h-2); r != Open {
			t.Fatalf("scroll click %v", r)
		}
		box, x, y = m.View(th, 80, 22)
	}
	if !strings.Contains(ansi.Strip(box), "Action 29") {
		t.Fatalf("cannot reach last item:\n%s", box)
	}
	for row, text := range strings.Split(ansi.Strip(box), "\n") {
		if strings.Contains(text, "Action 29") {
			if r := m.Click(th, 80, 22, x+2, y+row); r != Chosen || m.Action() != "29" {
				t.Fatalf("scrolled click chose %q (%v)", m.Action(), r)
			}
		}
	}
	m = NewPane(items, 79, 21)
	m.Key(tea.Key{Code: tea.KeyUp})
	box, x, y = m.View(th, 80, 22)
	if !strings.Contains(ansi.Strip(box), "Action 29") || m.Selected() != 29 || x < 0 || y < 0 {
		t.Fatalf("keyboard wrap failed:\n%s", box)
	}
	// Selection and hit mapping remain correct after a resize.
	box, x, y = m.View(th, 30, 8)
	if box == "" || strings.Count(box, "\n")+1 > 8 {
		t.Fatalf("resized menu:\n%s", box)
	}
	for row, text := range strings.Split(ansi.Strip(box), "\n") {
		if strings.Contains(text, "Action 29") {
			if r := m.Click(th, 30, 8, x+2, y+row); r != Chosen || m.Action() != "29" {
				t.Fatal("resize changed clicked item")
			}
		}
	}
}
