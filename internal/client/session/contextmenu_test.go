package session

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/pkg/api"
)

func rightClick(x, y int) tea.MouseClickMsg {
	return tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseRight}
}

func content(m *Model) string { return m.View().Content }

func TestRightClickOpensMenuWithShortcuts(t *testing.T) {
	m, r := threePaneScreen(t)
	m.Update(rightClick(50, 3)) // pane 2 already has the focus
	c := content(m)
	for _, want := range []string{"Split right", "Ctrl+b %", "Zoom", "Ctrl+b z", "Rename window", "Close pane", "Ctrl+b x"} {
		if !strings.Contains(c, want) {
			t.Errorf("menu lacks %q:\n%s", want, c)
		}
	}
	if len(r.sent) != 0 {
		t.Fatalf("sent %v, want nothing yet", r.sent)
	}
}

func TestRightClickFocusesTheClickedPane(t *testing.T) {
	m, r := threePaneScreen(t)
	m.Update(rightClick(5, 5))
	if cmd := lastCommand(t, r); cmd.Op != api.OpFocus || cmd.Target != 1 {
		t.Fatalf("sent %+v, want focus pane 1", cmd)
	}
	if m.menu == nil {
		t.Fatal("no menu")
	}
}

func TestRightClickElsewhereDoesNothing(t *testing.T) {
	m, r := threePaneScreen(t)
	m.Update(rightClick(40, 5))
	m.Update(rightClick(70, 23))
	m.Update(rightClick(15, 23))
	if m.menu != nil || len(r.sent) != 0 {
		t.Fatalf("menu %v, sent %v", m.menu != nil, r.sent)
	}
}

// chooseByClick opens the menu at 30,3 and clicks the item that shows label.
func chooseByClick(t *testing.T, m *Model, label string) {
	t.Helper()
	m.Update(rightClick(50, 3))
	for i, row := range strings.Split(ansi.Strip(content(m)), "\n") {
		if x := strings.Index(row, label); x >= 0 {
			m.Update(click(ansi.StringWidth(row[:x]), i))
			return
		}
	}
	t.Fatalf("no menu row %q:\n%s", label, content(m))
}

func TestMenuItemsRunTheirActions(t *testing.T) {
	m, _ := pickerScreen(t, 80, 24)
	chooseByClick(t, m, "Split right")
	if m.picker == nil || m.pending.Op != api.OpSplit || m.pending.Args[api.ArgDir] != api.DirRight || m.menu != nil {
		t.Fatalf("Split right: picker %v pending %+v menu %v", m.picker != nil, m.pending, m.menu != nil)
	}

	m, _ = pickerScreen(t, 80, 24)
	chooseByClick(t, m, "Split down")
	if m.picker == nil || m.pending.Args[api.ArgDir] != api.DirDown {
		t.Fatal("Split down did not open the picker")
	}

	for label, op := range map[string]string{"Zoom": api.OpZoom, "Even out": api.OpEven} {
		m, r := pickerScreen(t, 80, 24)
		chooseByClick(t, m, label)
		if c := lastCommand(t, r); c.Op != op || c.Target != 2 {
			t.Errorf("%s sent %+v, want %s on the focused pane 2", label, c, op)
		}
		if m.menu != nil {
			t.Errorf("%s left the menu open", label)
		}
	}

	m, _ = pickerScreen(t, 80, 24)
	chooseByClick(t, m, "Rename window")
	if m.rename == nil {
		t.Fatal("Rename window did not start the prompt")
	}
}

func TestMenuByKeyboard(t *testing.T) {
	m, r := pickerScreen(t, 80, 24)
	m.Update(rightClick(50, 3))
	m.Update(press(tea.KeyDown, 0, ""))
	m.Update(press(tea.KeyDown, 0, ""))
	m.Update(press(tea.KeyDown, 0, ""))
	m.Update(enter)
	if c := lastCommand(t, r); c.Op != api.OpZoom {
		t.Fatalf("sent %+v, want zoom", c)
	}
	m.Update(rightClick(50, 3))
	m.Update(press('z', 0, "z")) // keys do not reach the pane
	m.Update(press(tea.KeyEscape, 0, ""))
	if m.menu != nil || len(r.keys()) != 0 {
		t.Fatalf("menu open %v, keys sent %v", m.menu != nil, r.keys())
	}
}

func TestMenuDismissed(t *testing.T) {
	for name, dismiss := range map[string]tea.Msg{
		"click outside": click(5, 20), "right click": rightClick(5, 5),
	} {
		m, r := pickerScreen(t, 80, 24)
		m.Update(rightClick(50, 3))
		m.Update(dismiss)
		if m.menu != nil || len(r.sent) != 0 {
			t.Errorf("%s: menu open %v, sent %v", name, m.menu != nil, r.sent)
		}
	}
	m, _ := pickerScreen(t, 80, 24)
	m.Update(rightClick(50, 3))
	m.Update(click(51, 3)) // the top border
	if m.menu == nil {
		t.Error("a click on the frame closed the menu")
	}
}

// TestMenuShowsRebinding: after rebinding, the menu shows the new key.
func TestMenuShowsRebinding(t *testing.T) {
	m, _ := threePaneScreen(t)
	m.keys = rebound(t)
	m.Update(rightClick(50, 3))
	c := content(m)
	if !strings.Contains(c, "Ctrl+b |") || strings.Contains(c, "Ctrl+b %") {
		t.Fatalf("menu does not show the new key:\n%s", c)
	}
}

func TestProgramThatWantsTheMouseGetsTheRightClick(t *testing.T) {
	m, r := mousePane(t)
	m.Update(&api.PaneUpdate{Pane: 2, Mouse: true})
	m.Update(rightClick(50, 3)) // pane 2, focused
	if m.menu != nil {
		t.Fatal("the menu opened over a program that uses the mouse")
	}
	if ev := lastMouse(t, r); ev.Button != api.ButtonRight || ev.Pane != 2 {
		t.Fatalf("sent %+v, want the right press for pane 2", ev)
	}
	m.Update(tea.MouseClickMsg{X: 50, Y: 3, Button: tea.MouseRight, Mod: tea.ModCtrl})
	if m.menu == nil {
		t.Fatal("Ctrl+right click did not open the menu")
	}
}

func TestMenuNeedsRoom(t *testing.T) {
	m := screen(30, 6)
	panes := []api.PaneInfo{{ID: 1, Rect: api.Rect{W: 30, H: 5}, Focused: true}}
	m.Update(&api.Layout{Session: "s", Active: 1, Windows: []api.WindowInfo{{Index: 1, Panes: panes}}})
	m.Update(rightClick(3, 1))
	if m.menu != nil {
		t.Fatal("a menu that cannot show every item opened")
	}
}

func menuGolden(t *testing.T, cols, rows, x, y int) {
	m := screen(cols, rows)
	panes := threePanes(cols, rows)
	m.Update(&api.Layout{Session: "api", Active: 1, Windows: []api.WindowInfo{{Index: 1, Name: "dev", Panes: panes}}})
	fill(m, panes)
	m.Update(rightClick(x, y))
	golden.RequireEqual(t, m.View().Content)
}

func TestMenuGolden80x24(t *testing.T)      { menuGolden(t, 80, 24, 50, 3) }
func TestMenuGolden120x40(t *testing.T)     { menuGolden(t, 120, 40, 70, 5) }
func TestMenuEdgeGolden80x24(t *testing.T)  { menuGolden(t, 80, 24, 78, 21) }
func TestMenuEdgeGolden120x40(t *testing.T) { menuGolden(t, 120, 40, 118, 37) }

func TestMenuNewWindowCloseWindowAndDetach(t *testing.T) {
	m, _ := pickerScreen(t, 80, 24)
	chooseByClick(t, m, "New window")
	if m.picker == nil || m.pending.Op != api.OpNewWindow {
		t.Fatalf("New window: picker %v, pending %+v", m.picker != nil, m.pending)
	}

	m, r := pickerScreen(t, 80, 24)
	chooseByClick(t, m, "Close window")
	if m.confirm == nil || len(r.sent) != 0 {
		t.Fatalf("Close window: popup %v, sent %v; want the question first", m.confirm != nil, r.sent)
	}
	m.Update(press('y', 0, "y"))
	if c := lastCommand(t, r); c.Op != api.OpCloseWindow || c.Target != 1 {
		t.Fatalf("sent %+v, want close-window 1", c)
	}

	m, _ = pickerScreen(t, 80, 24)
	m.Update(rightClick(50, 3))
	// Up wraps to the last item, Detach.
	m.Update(press(tea.KeyUp, 0, ""))
	cmd := m.Update(enter)
	if cmd == nil {
		t.Fatal("Detach returned no command")
	}
	if _, ok := cmd().(DetachMsg); !ok {
		t.Fatal("Detach did not ask to detach")
	}
}
