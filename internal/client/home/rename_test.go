package home

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/pkg/api"
)

// TestRenameAsksForName: /rename on a picked session with no name asks for
// the name as a second step, starting from the current one (#87).
func TestRenameAsksForName(t *testing.T) {
	m := screen(80, 24, session("api", time.Minute, 0), session("web", time.Hour, 0))
	typeText(m, "/rename")
	m.Update(press(tea.KeyDown)) // api
	enter(m)
	if !strings.Contains(ansi.Strip(m.View().Content), "New name for api: api") {
		t.Fatalf("no prompt for api:\n%s", ansi.Strip(m.View().Content))
	}
	m.Update(tea.KeyPressMsg(tea.Key{Code: 'u', Mod: tea.ModCtrl}))
	typeText(m, "backend")
	s, ok := enter(m).(SendMsg)
	if r, rok := s.Msg.(*api.RenameSession); !ok || !rok || *r != (api.RenameSession{Session: "api", Name: "backend"}) {
		t.Fatalf("sent %#v, want rename api to backend", s.Msg)
	}
	if m.naming != nil || m.input != "" {
		t.Fatalf("prompt open %v, input %q after Enter", m.naming != nil, m.input)
	}
}

// TestRenameBadNameKeepsPrompt: a refused name shows why and keeps the
// prompt open.
func TestRenameBadNameKeepsPrompt(t *testing.T) {
	m := screen(80, 24, session("api", time.Minute, 0))
	typeText(m, "/rename")
	m.Update(press(tea.KeyDown))
	enter(m)
	m.Update(tea.KeyPressMsg(tea.Key{Code: 'u', Mod: tea.ModCtrl}))
	if got := enter(m); got != nil {
		t.Fatalf("Enter on an empty name gave %#v", got)
	}
	if m.naming == nil || !strings.Contains(m.notice, "empty") {
		t.Fatalf("prompt open %v, notice %q", m.naming != nil, m.notice)
	}
}

// TestRenameEscCancels: Esc sends nothing and goes back to picking the
// session.
func TestRenameEscCancels(t *testing.T) {
	m := screen(80, 24, session("api", time.Minute, 0))
	typeText(m, "/rename")
	m.Update(press(tea.KeyDown))
	enter(m)
	if cmd := m.Update(press(tea.KeyEscape)); cmd != nil {
		t.Fatalf("Esc gave %#v", msgOf(cmd))
	}
	if m.naming != nil || m.input != "/rename " || m.target != "api" {
		t.Fatalf("prompt open %v, input %q, target %q; want back at /rename with api", m.naming != nil, m.input, m.target)
	}
}

func TestRenameNameGolden80x24(t *testing.T) { goldenTyped(t, 80, 24, "/rename↓\r") }

// TestRenameGoneSession: a session killed elsewhere after it was picked is
// refused with a notice, not offered for a new name.
func TestRenameGoneSession(t *testing.T) {
	m := screen(80, 24, session("api", time.Minute, 0), session("web", time.Hour, 0))
	typeText(m, "/rename")
	m.Update(press(tea.KeyDown)) // api
	m.Update(&api.SessionList{Sessions: []api.SessionInfo{session("web", time.Hour, 0)}})
	enter(m)
	if m.naming != nil || !strings.Contains(m.notice, "No session named api") {
		t.Fatalf("prompt open %v, notice %q", m.naming != nil, m.notice)
	}
}

// TestRenameNoSessions: with no sessions, /rename says so.
func TestRenameNoSessions(t *testing.T) {
	m := screen(80, 24)
	typeText(m, "/rename")
	enter(m)
	if !strings.Contains(m.notice, "No sessions to rename") {
		t.Fatalf("notice %q", m.notice)
	}
}

// TestRenameLongNamePrompt: a long session name is cut in the prompt, so the
// typed name stays visible at 80 columns.
func TestRenameLongNamePrompt(t *testing.T) {
	long := strings.Repeat("x", api.MaxSessionName)
	m := screen(80, 24, session(long, time.Minute, 0))
	typeText(m, "/rename")
	m.Update(press(tea.KeyDown))
	enter(m)
	m.Update(tea.KeyPressMsg(tea.Key{Code: 'u', Mod: tea.ModCtrl}))
	typeText(m, "short")
	if !strings.Contains(ansi.Strip(m.View().Content), "short") {
		t.Fatalf("the typed name is hidden:\n%s", ansi.Strip(m.View().Content))
	}
}

// TestCtrlCQuitsFromNameStep: Ctrl+C leaves Snow from the name step, as from
// the rest of the page.
func TestCtrlCQuitsFromNameStep(t *testing.T) {
	m := screen(80, 24, session("api", time.Minute, 0))
	typeText(m, "/rename")
	m.Update(press(tea.KeyDown))
	enter(m)
	if got := msgOf(m.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))); got != (QuitMsg{}) {
		t.Fatalf("Ctrl+C gave %#v, want QuitMsg", got)
	}
}
