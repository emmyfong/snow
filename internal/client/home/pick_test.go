package home

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// TestCommandListKeepsSessions: the sessions stay on screen while a command
// is typed, unfiltered by the command text (#85).
func TestCommandListKeepsSessions(t *testing.T) {
	m := screen(80, 24, session("api", time.Minute, 0), session("web", time.Hour, 0))
	typeText(m, "/rename")
	view := ansi.Strip(m.View().Content)
	for _, want := range []string{"/rename", "Sessions", "api", "web"} {
		if !strings.Contains(view, want) {
			t.Errorf("view lacks %q:\n%s", want, view)
		}
	}
}

// TestCommandListShortScreen: on a short screen the sessions shrink first;
// the commands stay, and no line is wider than the screen.
func TestCommandListShortScreen(t *testing.T) {
	m := screen(80, 10, session("api", time.Minute, 0), session("web", time.Hour, 0))
	typeText(m, "/")
	view := m.View().Content
	plain := ansi.Strip(view)
	if !strings.Contains(plain, "/new") {
		t.Fatalf("the commands are gone on 80x10:\n%s", plain)
	}
	for i, l := range strings.Split(view, "\n") {
		if w := ansi.StringWidth(l); w > 80 {
			t.Errorf("line %d is %d cells wide", i, w)
		}
	}
}

// TestShortScreenKeepsSessions: from 11 rows up, a session row stays under
// the commands, so a pick is never blind. The wordmark and the commands give
// up rows first.
func TestShortScreenKeepsSessions(t *testing.T) {
	for _, rows := range []int{11, 14, 16, 20} {
		for typed, command := range map[string]string{"/": "/new", "/kill": "/kill"} {
			m := screen(80, rows, session("api", time.Minute, 0), session("web", time.Hour, 0))
			typeText(m, typed)
			plain := ansi.Strip(m.View().Content)
			if !strings.Contains(plain, "api") || !strings.Contains(plain, command) {
				t.Errorf("80x%d %q: want a command and a session row:\n%s", rows, typed, plain)
			}
		}
	}
}

// TestCommandKeepsTarget: the session highlighted before "/" stays marked
// under the commands.
func TestCommandKeepsTarget(t *testing.T) {
	m := screen(80, 24, session("api", time.Minute, 0), session("web", time.Hour, 0))
	m.Update(press(tea.KeyDown))
	m.Update(press(tea.KeyDown)) // web
	typeText(m, "/kill")
	p := m.layout()
	if p.sessions.lines == 0 {
		t.Fatal("no session rows under the commands")
	}
	row := strings.Split(ansi.Strip(m.View().Content), "\n")[p.sessions.top+1]
	if !strings.Contains(row, theme.Default().Glyphs.Pointer) || !strings.Contains(row, "web") {
		t.Fatalf("row %q, want web marked", row)
	}
}

// TestClickOnSessionWhileCommanding: under a command that does not act on
// a session, a click on a session row does nothing and keeps the input.
func TestClickOnSessionWhileCommanding(t *testing.T) {
	m := screen(80, 24, session("api", time.Minute, 0), session("web", time.Hour, 0))
	typeText(m, "/new")
	p := m.layout()
	cmd := m.Update(tea.MouseClickMsg{X: p.x + 2, Y: p.sessions.top + 1, Button: tea.MouseLeft})
	if cmd != nil || m.input != "/new" {
		t.Fatalf("click gave %v, input %q", cmd, m.input)
	}
}

// TestKillPickedFromList: after /kill, the arrows pick the session and
// Enter asks to kill it (#86).
func TestKillPickedFromList(t *testing.T) {
	m := screen(80, 24, session("api", time.Minute, 0), session("web", time.Hour, 0))
	typeText(m, "/kill")
	m.Update(press(tea.KeyDown)) // api
	m.Update(press(tea.KeyDown)) // web
	enter(m)
	if m.confirm == nil || m.killing != "web" {
		t.Fatalf("killing %q, confirm open %v; want web", m.killing, m.confirm != nil)
	}
}

// TestRenamePickedFromList: the session picked by arrows is renamed to the
// typed name.
func TestRenamePickedFromList(t *testing.T) {
	m := screen(80, 24, session("api", time.Minute, 0), session("web", time.Hour, 0))
	typeText(m, "/rename ")
	m.Update(press(tea.KeyDown)) // api
	typeText(m, "backend")
	s, ok := enter(m).(SendMsg)
	if r, rok := s.Msg.(*api.RenameSession); !ok || !rok || *r != (api.RenameSession{Session: "api", Name: "backend"}) {
		t.Fatalf("sent %#v, want rename api to backend", s.Msg)
	}
}

// TestPartialWordMovesCommands: while the command word is partial, the
// arrows move in the command list, not the sessions.
func TestPartialWordMovesCommands(t *testing.T) {
	m := screen(80, 24, session("api", time.Minute, 0))
	typeText(m, "/")
	m.Update(press(tea.KeyDown))
	if m.cmdSel != 1 || m.target != "" {
		t.Fatalf("cmdSel %d, target %q; want the command highlight to move", m.cmdSel, m.target)
	}
}

// TestClickPicksSession: under /kill, a click on a session row picks it and
// asks to kill it.
func TestClickPicksSession(t *testing.T) {
	m := screen(80, 24, session("api", time.Minute, 0), session("web", time.Hour, 0))
	typeText(m, "/kill")
	p := m.layout()
	m.Update(tea.MouseClickMsg{X: p.x + 2, Y: p.sessions.top + 1, Button: tea.MouseLeft})
	if m.confirm == nil || m.killing != "web" {
		t.Fatalf("killing %q, confirm open %v; want web", m.killing, m.confirm != nil)
	}
}

func TestPickedGolden80x24(t *testing.T) { goldenTyped(t, 80, 24, "/kill↓↓") }
func TestPickedGolden80x14(t *testing.T) { goldenTyped(t, 80, 14, "/kill↓↓") }
