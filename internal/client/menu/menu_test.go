package menu

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/keys"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/config"
)

func rebound(t *testing.T) *keys.Table {
	t.Helper()
	cfg := config.Defaults().Keys
	cfg.Prefix = "C-a"
	cfg.Bind = map[string]string{"|": "split-right", "%": config.UnbindAction}
	tab, err := keys.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return tab
}

func TestPaneMenuItems(t *testing.T) {
	items := PaneItems(keys.Default())
	want := []Item{
		{"Split right", "Ctrl+b %", "split-right"},
		{"Split down", `Ctrl+b "`, "split-down"},
		{"Zoom", "Ctrl+b z", "zoom"},
		{"Even out", "Ctrl+b E", "even-out"},
		{"New window", "Ctrl+b c", "new-window"},
		{"Rename window", "Ctrl+b ,", "rename-window"},
		{"Close pane", "Ctrl+b x", "close-pane"},
		{"Close window", "Ctrl+b &", "close-window"},
		{"Switch session", "Ctrl+b s", "sessions"},
		{"Detach", "Ctrl+b d", "detach"},
	}
	if len(items) != len(want) {
		t.Fatalf("%d items, want %d: %+v", len(items), len(want), items)
	}
	for i := range want {
		if items[i] != want[i] {
			t.Errorf("item %d = %+v, want %+v", i, items[i], want[i])
		}
	}
}

// TestMenuShowsRebinding: after rebinding, the menu shows the new key.
func TestMenuShowsRebinding(t *testing.T) {
	m := New(PaneItems(rebound(t)), 10, 5)
	box, _, _ := m.View(theme.Default(), 80, 23)
	text := ansi.Strip(box)
	if !strings.Contains(text, "Split right") || !strings.Contains(text, "Ctrl+a |") {
		t.Fatalf("menu does not show the new binding:\n%s", text)
	}
	if strings.Contains(text, "%") || strings.Contains(text, "Ctrl+b") {
		t.Fatalf("menu still shows an old key:\n%s", text)
	}
}

func TestUnboundActionShowsNoShortcut(t *testing.T) {
	cfg := config.Defaults().Keys
	cfg.Bind = map[string]string{"z": config.UnbindAction}
	tab, err := keys.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range PaneItems(tab) {
		if it.Action == "zoom" && it.Shortcut != "" {
			t.Errorf("zoom has the shortcut %q after it was unbound", it.Shortcut)
		}
	}
}

func TestPlacementStaysOnScreen(t *testing.T) {
	th := theme.Default()
	items := PaneItems(keys.Default())
	for _, size := range [][2]int{{80, 23}, {120, 39}, {30, 8}, {10, 4}} {
		for _, at := range [][2]int{{0, 0}, {40, 10}, {79, 22}, {119, 39}, {5, 100}, {100, 3}} {
			m := New(items, at[0], at[1])
			box, x, y := m.View(th, size[0], size[1])
			if box == "" {
				continue
			}
			w, h := ansi.StringWidth(strings.SplitN(box, "\n", 2)[0]), strings.Count(box, "\n")+1
			if x < 0 || y < 0 || x+w > size[0] || y+h > size[1] {
				t.Errorf("screen %v click %v: box %dx%d at %d,%d leaves the screen", size, at, w, h, x, y)
			}
		}
	}
}

func TestPlacementStartsAtTheClick(t *testing.T) {
	m := New(PaneItems(keys.Default()), 12, 3)
	_, x, y := m.View(theme.Default(), 80, 23)
	if x != 12 || y != 3 {
		t.Fatalf("placed at %d,%d, want the click at 12,3", x, y)
	}
	m = New(PaneItems(keys.Default()), 79, 22)
	box, x, y := m.View(theme.Default(), 80, 23)
	w := ansi.StringWidth(strings.SplitN(box, "\n", 2)[0])
	h := len(PaneItems(keys.Default())) + 2
	if x != 80-w || y != 23-h {
		t.Fatalf("clamped to %d,%d, want %d,%d", x, y, 80-w, 23-h)
	}
}

func TestKeyNavigation(t *testing.T) {
	m := New(PaneItems(keys.Default()), 0, 0)
	if m.Selected() != 0 {
		t.Fatalf("starts at %d, want the first item", m.Selected())
	}
	for _, k := range []tea.Key{{Code: tea.KeyDown}, {Code: 'j', Text: "j"}} {
		m.Key(k)
	}
	if m.Selected() != 2 {
		t.Fatalf("after two downs: %d, want 2", m.Selected())
	}
	m.Key(tea.Key{Code: tea.KeyUp})
	m.Key(tea.Key{Code: 'k', Text: "k"})
	m.Key(tea.Key{Code: tea.KeyUp})
	if m.Selected() != len(m.items)-1 {
		t.Fatalf("up from the top: %d, want the last item (wraps)", m.Selected())
	}
	m.Key(tea.Key{Code: tea.KeyDown})
	if m.Selected() != 0 {
		t.Fatalf("down from the bottom: %d, want the first item (wraps)", m.Selected())
	}
	m.Key(tea.Key{Code: tea.KeyDown})
	if r := m.Key(tea.Key{Code: tea.KeyEnter}); r != Chosen || m.Action() != "split-down" {
		t.Fatalf("Enter = %v on %q, want Chosen split-down", r, m.Action())
	}
	if r := m.Key(tea.Key{Code: tea.KeyEscape}); r != Closed {
		t.Fatalf("Escape = %v, want Closed", r)
	}
	if r := m.Key(tea.Key{Code: 'x', Text: "x"}); r != Open {
		t.Fatalf("a stray key = %v, want Open", r)
	}
}

func TestEmptyMenu(t *testing.T) {
	m := New(nil, 3, 3)
	if box, _, _ := m.View(theme.Default(), 80, 23); box != "" {
		t.Errorf("empty menu drew %q", box)
	}
	if r := m.Key(tea.Key{Code: tea.KeyEnter}); r != Closed {
		t.Errorf("Enter on an empty menu = %v, want Closed", r)
	}
	if r := m.Key(tea.Key{Code: tea.KeyDown}); r != Open {
		t.Errorf("Down on an empty menu = %v", r)
	}
}

func TestClick(t *testing.T) {
	th := theme.Default()
	m := New(PaneItems(keys.Default()), 10, 4)
	box, x, y := m.View(th, 80, 23)
	w, h := ansi.StringWidth(strings.SplitN(box, "\n", 2)[0]), strings.Count(box, "\n")+1
	tests := []struct {
		name   string
		cx, cy int
		want   Result
		action string
	}{
		{"first item", x + 3, y + 1, Chosen, "split-right"},
		{"third item, last column", x + w - 2, y + 3, Chosen, "zoom"},
		{"last item", x + 1, y + h - 2, Chosen, "detach"},
		{"top border", x + 3, y, Open, ""},
		{"bottom border", x + 3, y + h - 1, Open, ""},
		{"left border", x, y + 2, Open, ""},
		{"right border", x + w - 1, y + 2, Open, ""},
		{"outside", x - 1, y + 2, Closed, ""},
		{"far outside", 70, 20, Closed, ""},
	}
	for _, tt := range tests {
		m := New(PaneItems(keys.Default()), 10, 4)
		got := m.Click(th, 80, 23, tt.cx, tt.cy)
		if got != tt.want || (tt.want == Chosen && m.Action() != tt.action) {
			t.Errorf("%s: Click(%d,%d) = %v on %q, want %v on %q", tt.name, tt.cx, tt.cy, got, m.Action(), tt.want, tt.action)
		}
	}
}

func TestViewIsWidthSafe(t *testing.T) {
	th := theme.Default()
	for w := 0; w <= 40; w += 3 {
		for h := 0; h <= 12; h += 2 {
			box, _, _ := New(PaneItems(keys.Default()), 1, 1).View(th, w, h)
			for _, r := range strings.Split(box, "\n") {
				if box != "" && ansi.StringWidth(r) > w {
					t.Fatalf("screen %dx%d: row %q is %d wide", w, h, r, ansi.StringWidth(r))
				}
			}
			if box != "" && strings.Count(box, "\n")+1 > h {
				t.Fatalf("screen %dx%d: box is %d rows", w, h, strings.Count(box, "\n")+1)
			}
		}
	}
}
