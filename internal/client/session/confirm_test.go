package session

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/pkg/api"
)

// clickLabel clicks the first screen cell of label.
func clickLabel(t *testing.T, m *Model, label string, msg func(x, y int) tea.Msg) {
	t.Helper()
	for i, row := range strings.Split(ansi.Strip(content(m)), "\n") {
		if x := strings.Index(row, label); x >= 0 {
			m.Update(msg(ansi.StringWidth(row[:x])+1, i))
			return
		}
	}
	t.Fatalf("no %q on screen:\n%s", label, content(m))
}

func leftClick(x, y int) tea.Msg { return click(x, y) }

// TestMenuClosePaneConfirmsWithTheMouse: the menu's Close pane asks in a
// popup the mouse can answer.
func TestMenuClosePaneConfirmsWithTheMouse(t *testing.T) {
	m, r := pickerScreen(t, 80, 24)
	chooseByClick(t, m, "Close pane")
	if len(r.sent) != 0 || m.confirm == nil {
		t.Fatalf("sent %v, confirm open %v; want a question first", r.sent, m.confirm != nil)
	}
	clickLabel(t, m, "[ Close ]", leftClick)
	if c := lastCommand(t, r); c.Op != api.OpClose || c.Target != 2 {
		t.Fatalf("sent %+v, want close pane 2", c)
	}
	if m.confirm != nil {
		t.Fatal("the popup stayed open")
	}
}

func TestConfirmCancelledByMouseAndKeys(t *testing.T) {
	for name, cancel := range map[string]func(*Model){
		"cancel button": func(m *Model) { clickLabel(t, m, "[ Cancel ]", leftClick) },
		"click outside": func(m *Model) { m.Update(click(1, 1)) },
		"n":             func(m *Model) { m.Update(press('n', 0, "n")) },
		"escape":        func(m *Model) { m.Update(press(tea.KeyEscape, 0, "")) },
		"enter":         func(m *Model) { m.Update(enter) },
	} {
		m, r := pickerScreen(t, 80, 24)
		m.Update(ctrlB)
		m.Update(press('x', 0, "x"))
		cancel(m)
		if m.confirm != nil || len(r.sent) != 0 {
			t.Errorf("%s: popup open %v, sent %v", name, m.confirm != nil, r.sent)
		}
	}
}

func TestConfirmKeysAndMouseShareOnePopup(t *testing.T) {
	m, r := pickerScreen(t, 80, 24)
	m.Update(ctrlB)
	m.Update(press('x', 0, "x"))
	m.Update(click(70, 5)) // the question stays; a stray click elsewhere cancels
	if m.confirm != nil {
		t.Fatal("click outside left the popup open")
	}
	chooseByClick(t, m, "Close pane")
	if got := content(m); !strings.Contains(got, "Close this pane") {
		t.Fatalf("menu did not show the same question:\n%s", got)
	}
	if len(r.sent) != 0 {
		t.Fatalf("sent %v", r.sent)
	}
}

func TestMiddleClickTabClosesWindowAfterConfirming(t *testing.T) {
	m, r := threePaneScreen(t)
	m.Update(tea.MouseClickMsg{X: 15, Y: 23, Button: tea.MouseMiddle}) // tab 2, "logs"
	if len(r.sent) != 0 || m.confirm == nil {
		t.Fatalf("sent %v, popup %v; want the question first", r.sent, m.confirm != nil)
	}
	if got := content(m); !strings.Contains(got, `Close window 2 "logs" and its 0 panes?`) {
		t.Fatalf("popup does not name window 2:\n%s", got)
	}
	m.Update(press('y', 0, "y"))
	if c := lastCommand(t, r); c.Op != api.OpCloseWindow || c.Target != 2 {
		t.Fatalf("sent %+v, want close-window 2", c)
	}
}

func TestMiddleClickOutsideTabsDoesNothing(t *testing.T) {
	m, r := threePaneScreen(t)
	m.Update(tea.MouseClickMsg{X: 70, Y: 23, Button: tea.MouseMiddle})
	m.Update(tea.MouseClickMsg{X: 5, Y: 5, Button: tea.MouseMiddle})
	if m.confirm != nil || len(r.sent) != 0 {
		t.Fatalf("popup %v, sent %v", m.confirm != nil, r.sent)
	}
}

func TestPopupHoldsKeysAndPaste(t *testing.T) {
	m, r := pickerScreen(t, 80, 24)
	m.Update(ctrlB)
	m.Update(press('x', 0, "x"))
	m.Update(press('l', 0, "l"))
	m.Update(tea.PasteMsg{Content: "hi"})
	m.Update(tea.MouseWheelMsg{X: 5, Y: 5, Button: tea.MouseWheelUp})
	if len(r.sent) != 0 {
		t.Fatalf("sent %v while the question was open", r.sent)
	}
}

func confirmGolden(t *testing.T, cols, rows int) {
	m := screen(cols, rows)
	panes := threePanes(cols, rows)
	panes[1].Folder = "/src"
	m.Update(&api.Layout{Session: "api", Active: 1, Windows: []api.WindowInfo{{Index: 1, Name: "dev", Panes: panes}}})
	fill(m, panes)
	m.Update(ctrlB)
	m.Update(press('x', 0, "x"))
	golden.RequireEqual(t, m.View().Content)
}

func TestConfirmGolden80x24(t *testing.T)  { confirmGolden(t, 80, 24) }
func TestConfirmGolden120x40(t *testing.T) { confirmGolden(t, 120, 40) }
