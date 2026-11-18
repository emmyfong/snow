package home

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestOpenNewSessionFromEnter(t *testing.T) {
	m := screen(80, 24)
	m.SetFolder("/work/api")
	if cmd := m.Update(press(tea.KeyEnter)); cmd != nil || m.dialog == nil {
		t.Fatalf("empty Enter: dialog open %v, command %v; want dialog and no command", m.dialog != nil, cmd)
	}
	if got := m.dialog.Choice().Folder; got != "/work/api" {
		t.Fatalf("dialog folder = %q, want /work/api", got)
	}
	typeText(m, "work")
	if cmd := m.Update(press(tea.KeyEnter)); cmd == nil {
		t.Fatal("dialog Enter returned no OpenMsg command")
	} else if got, ok := cmd().(OpenMsg); !ok || got != (OpenMsg{Create: true, Dir: "/work/apiwork"}) {
		t.Fatalf("dialog Enter produced %#v, want numbered session in edited folder", got)
	}
}

func TestNewSessionEscCloses(t *testing.T) {
	m := screen(80, 24)
	before := m.View().Content
	m.openNewSession()
	if cmd := m.Update(press(tea.KeyEscape)); cmd != nil || m.dialog != nil {
		t.Fatalf("Esc: dialog open %v, command %v; want closed without quitting", m.dialog != nil, cmd)
	}
	if got := m.View().Content; got != before {
		t.Fatal("closing the dialog changed the home page underneath it")
	}
}

func TestNewSessionTooSmallShowsNotice(t *testing.T) {
	m := screen(20, 10)
	m.openNewSession()
	if m.dialog != nil || m.notice == "" {
		t.Fatalf("too-small window: dialog %v, notice %q", m.dialog != nil, m.notice)
	}
}

func TestNewSessionCommand(t *testing.T) {
	m := screen(80, 24, session("work", 0, 0))
	typeText(m, "/new")
	if cmd := m.Update(press(tea.KeyEnter)); cmd != nil || m.dialog == nil {
		t.Fatalf("/new: dialog open %v, command %v; want dialog and no command", m.dialog != nil, cmd)
	}

	m = screen(80, 24)
	typeText(m, "/new work")
	if got := open(t, m.Update(press(tea.KeyEnter))); got != (OpenMsg{Session: "work", Create: true}) {
		t.Fatalf("/new work opened %+v, want named session", got)
	}
}

func TestNewSessionClickAcceptsChoice(t *testing.T) {
	m := screen(80, 24)
	m.SetFolder("/work")
	m.dialog = nil
	m.openNewSession()
	box, x, y := m.dialog.View(m.theme, 80, 24)
	if box == "" {
		t.Fatal("new-session dialog did not render")
	}
	// Enter in the footer accepts by mouse, matching the keyboard action.
	plain := strings.Split(ansi.Strip(box), "\n")
	var clickX, clickY int
	for row, line := range plain {
		if col := strings.Index(line, "Enter start"); col >= 0 {
			clickX, clickY = x+col+2, y+row
			break
		}
	}
	if clickY == 0 {
		t.Fatal("dialog has no Enter hint")
	}
	if cmd := m.Update(tea.MouseClickMsg{X: clickX, Y: clickY, Button: tea.MouseLeft}); cmd == nil {
		t.Fatal("clicking the Enter hint returned no OpenMsg")
	} else if got, ok := cmd().(OpenMsg); !ok || got != (OpenMsg{Create: true, Dir: "/work"}) {
		t.Fatalf("click produced %#v, want numbered session in /work", got)
	}
}
