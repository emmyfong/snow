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

func TestPrefixBindings(t *testing.T) {
	tests := []struct {
		key  tea.KeyPressMsg
		op   string
		args map[string]string
		win  int
	}{
		{press('x', 0, "x"), api.OpClose, nil, 0},
		{press('z', 0, "z"), api.OpZoom, nil, 0},
		{press('E', 0, "E"), api.OpEven, nil, 0},
		{press(tea.KeyLeft, 0, ""), api.OpFocus, map[string]string{api.ArgDir: api.DirLeft}, 0},
		{press(tea.KeyUp, tea.ModCtrl, ""), api.OpResize, map[string]string{api.ArgDir: api.DirUp, api.ArgCells: "1"}, 0},
		{press(tea.KeyRight, tea.ModAlt, ""), api.OpResize, map[string]string{api.ArgDir: api.DirRight, api.ArgCells: "5"}, 0},
		{press('n', 0, "n"), api.OpNextWindow, nil, -1},
		{press('p', 0, "p"), api.OpPrevWindow, nil, -1},
		{press('3', 0, "3"), api.OpSelectWindow, nil, 3},
		{press('0', 0, "0"), api.OpSelectWindow, nil, 10},
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
		case 0:
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
