package session

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/pkg/api"
)

func lastCommand(t *testing.T, r *recorder) *api.Command {
	t.Helper()
	if len(r.sent) == 0 {
		t.Fatal("nothing sent")
	}
	c, ok := r.sent[len(r.sent)-1].(*api.Command)
	if !ok {
		t.Fatalf("sent %T, want *api.Command", r.sent[len(r.sent)-1])
	}
	return c
}

// pane marks a binding whose target is the focused pane.
const pane = -2

func TestPrefixBindings(t *testing.T) {
	tests := []struct {
		key  tea.KeyPressMsg
		op   string
		args map[string]string
		win  int // window index for window commands; pane means the focused pane
	}{
		{press('z', 0, "z"), api.OpZoom, nil, pane},
		{press('E', 0, "E"), api.OpEven, nil, pane},
		{press(tea.KeyLeft, 0, ""), api.OpFocus, map[string]string{api.ArgDir: api.DirLeft}, pane},
		{press(tea.KeyUp, tea.ModCtrl, ""), api.OpResize, map[string]string{api.ArgDir: api.DirUp, api.ArgCells: "1"}, pane},
		{press(tea.KeyRight, tea.ModAlt, ""), api.OpResize, map[string]string{api.ArgDir: api.DirRight, api.ArgCells: "5"}, pane},
		{press('n', 0, "n"), api.OpNextWindow, nil, -1},
		{press('p', 0, "p"), api.OpPrevWindow, nil, -1},
		{press('3', 0, "3"), api.OpSelectWindow, nil, 3},
		{press('0', 0, "0"), api.OpSelectWindow, nil, 0},
	}
	for _, tt := range tests {
		m, r := newPrefixScreen(t)
		m.Update(ctrlB)
		m.Update(tt.key)
		c := lastCommand(t, r)
		if c.Op != tt.op {
			t.Errorf("%v: op %q, want %q", tt.key, c.Op, tt.op)
			continue
		}
		switch tt.win {
		case pane:
			if c.Target != 7 {
				t.Errorf("%v: target %d, want the focused pane 7", tt.key, c.Target)
			}
		case -1:
		default:
			if c.Target != tt.win {
				t.Errorf("%v: target %d, want window %d", tt.key, c.Target, tt.win)
			}
		}
		for k, v := range tt.args {
			if c.Args[k] != v {
				t.Errorf("%v: arg %s = %q, want %q", tt.key, k, c.Args[k], v)
			}
		}
	}
}

func TestRefusedCommandShowsInStatusBar(t *testing.T) {
	m := attached(t, 80, 24)
	cmd := m.Update(&api.Error{Code: api.CodeCommandFailed, Message: "no room to split"})
	rows := strings.Split(m.View().Content, "\n")
	if bar := ansi.Strip(rows[len(rows)-1]); !strings.Contains(bar, "no room to split") {
		t.Fatalf("status bar %q, want the refusal", bar)
	}
	if cmd == nil {
		t.Fatal("no timer to clear the message")
	}
	m.Update(cmd())
	rows = strings.Split(m.View().Content, "\n")
	if bar := ansi.Strip(rows[len(rows)-1]); strings.Contains(bar, "no room") {
		t.Fatalf("message still shown after its timer: %q", bar)
	}
}

func TestCursorIsOffsetByThePaneRect(t *testing.T) {
	m := screen(80, 24)
	panes := threePanes(80, 24)
	m.Update(&api.Layout{Session: "api", Active: 1, Windows: []api.WindowInfo{{Index: 1, Panes: panes}}})
	m.Update(&api.PaneUpdate{Pane: 2, Cursor: &api.Cursor{X: 3, Y: 1, Visible: true}})
	c := m.View().Cursor
	if c == nil || c.X != panes[1].Rect.X+3 || c.Y != 1 {
		t.Fatalf("cursor %+v, want pane 2's origin plus 3,1", c)
	}
}

func statusLine(m *Model) string {
	rows := strings.Split(m.View().Content, "\n")
	return ansi.Strip(rows[len(rows)-1])
}

// TestCloseAsksFirst: closing a pane or a window always asks, because it
// ends the programs in it.
func TestCloseAsksFirst(t *testing.T) {
	for _, tt := range []struct {
		key      tea.KeyPressMsg
		question string
		op       string
		target   int
	}{
		{press('x', 0, "x"), "close this pane (1 of 1)? y/n", api.OpClose, 7},
		{press('&', 0, "&"), "close window 1 and its 1 pane? y/n", api.OpCloseWindow, 1},
	} {
		m, r := newPrefixScreen(t)
		m.Update(ctrlB)
		m.Update(tt.key)
		if len(r.sent) != 0 {
			t.Fatalf("%v sent %v before the answer", tt.key, r.sent)
		}
		if got := statusLine(m); !strings.Contains(got, tt.question) {
			t.Fatalf("status %q, want the question %q", got, tt.question)
		}
		m.Update(press('y', 0, "y"))
		c := lastCommand(t, r)
		if c.Op != tt.op || c.Target != tt.target {
			t.Fatalf("y sent %+v, want %s on %d", c, tt.op, tt.target)
		}
		if strings.Contains(statusLine(m), "y/n") {
			t.Fatal("the question stayed after the answer")
		}
	}
}

func TestCloseCancelledByAnyOtherKey(t *testing.T) {
	m, r := newPrefixScreen(t)
	m.Update(ctrlB)
	m.Update(press('x', 0, "x"))
	m.Update(press('n', 0, "n"))
	m.Update(press('l', 0, "l"))
	if got := r.keys(); len(r.sent) != 1 || got[0] != "l" {
		t.Fatalf("sent %v, want only the key l after the cancelled close", r.sent)
	}
}

// TestCloseQuestionNamesThePane: pane IDs only grow, so the question names
// the pane by its program, folder, and place in the window.
func TestCloseQuestionNamesThePane(t *testing.T) {
	m, _ := pickerScreen(t, 80, 24)
	m.layout.Windows[0].Name = "logs"
	m.Update(ctrlB)
	m.Update(press('x', 0, "x"))
	if got := statusLine(m); !strings.Contains(got, "close this pane (Ubuntu /src, 2 of 3)? y/n") {
		t.Fatalf("status %q, want the pane named by profile, folder, and place", got)
	}
	m.Update(press('n', 0, "n"))
	m.Update(ctrlB)
	m.Update(press('&', 0, "&"))
	if got := statusLine(m); !strings.Contains(got, `close window 1 "logs" and its 3 panes? y/n`) {
		t.Fatalf("status %q, want the window named with its pane count", got)
	}
}
