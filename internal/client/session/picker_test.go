package session

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/pkg/api"
)

// pickerScreen is a two-profile session whose focused pane runs Ubuntu in
// /src.
func pickerScreen(t *testing.T, cols, rows int) (*Model, *recorder) {
	t.Helper()
	r := &recorder{}
	m := screen(cols, rows)
	m.send = r.send
	m.Update(&api.ProfileList{Profiles: []string{"PowerShell", "Ubuntu", "cmd"}, Default: "PowerShell"})
	panes := threePanes(cols, rows)
	panes[1].Folder = "/src"
	m.Update(&api.Layout{Session: "api", Active: 1, Windows: []api.WindowInfo{{Index: 1, Name: "dev", Panes: panes}}})
	fill(m, panes)
	return m, r
}

var enter = press(tea.KeyEnter, 0, "")

func TestPickerDefaultsToCurrent(t *testing.T) {
	for _, tt := range []struct {
		key tea.KeyPressMsg
		op  string
		dir string
	}{
		{press('%', 0, "%"), api.OpSplit, api.DirRight},
		{press('"', 0, `"`), api.OpSplit, api.DirDown},
		{press('c', 0, "c"), api.OpNewWindow, ""},
	} {
		m, r := pickerScreen(t, 80, 24)
		m.Update(ctrlB)
		m.Update(tt.key)
		if len(r.sent) != 0 {
			t.Fatalf("%v sent %v before the picker was answered", tt.key, r.sent)
		}
		m.Update(enter)
		c := lastCommand(t, r)
		if c.Op != tt.op || c.Args[api.ArgDir] != tt.dir || c.Args[api.ArgProfile] != "Ubuntu" || c.Args[api.ArgFolder] != "/src" {
			t.Fatalf("%v then Enter sent %+v, want %s %s with Ubuntu in /src", tt.key, c, tt.op, tt.dir)
		}
		if tt.op == api.OpSplit && c.Target != 2 {
			t.Fatalf("split target %d, want the focused pane 2", c.Target)
		}
	}
}

func TestPickerChoosesAnotherProfile(t *testing.T) {
	m, r := pickerScreen(t, 80, 24)
	m.Update(ctrlB)
	m.Update(press('%', 0, "%"))
	m.Update(press(tea.KeyDown, 0, ""))
	m.Update(enter)
	if c := lastCommand(t, r); c.Args[api.ArgProfile] != "cmd" {
		t.Fatalf("profile %q, want cmd", c.Args[api.ArgProfile])
	}
}

func TestPickerEscapeCancelsAndKeysReturnToThePane(t *testing.T) {
	m, r := pickerScreen(t, 80, 24)
	m.Update(ctrlB)
	m.Update(press('%', 0, "%"))
	m.Update(press(tea.KeyEscape, 0, ""))
	m.Update(press('l', 0, "l"))
	if len(r.sent) != 1 || r.keys()[0] != "l" {
		t.Fatalf("sent %v, want only the key l after the cancelled picker", r.sent)
	}
}

func goldenPicker(t *testing.T, cols, rows int) {
	m, _ := pickerScreen(t, cols, rows)
	m.Update(ctrlB)
	m.Update(press('%', 0, "%"))
	golden.RequireEqual(t, m.View().Content)
}

func TestPickerGolden80x24(t *testing.T)  { goldenPicker(t, 80, 24) }
func TestPickerGolden120x40(t *testing.T) { goldenPicker(t, 120, 40) }
