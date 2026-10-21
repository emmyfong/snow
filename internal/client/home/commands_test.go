package home

import (
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

func TestFuzzyMatch(t *testing.T) {
	tests := []struct {
		query string
		first string
	}{
		{"ses", "sessions"},
		{"re", "rename"},
		{"k", "kill"},
		{"q", "quit"},
		{"nw", "new"},
		{"", "new"}, // no query keeps the table order
	}
	for _, tt := range tests {
		got := matchCommands(tt.query)
		if len(got) == 0 || got[0].name != tt.first {
			t.Errorf("matchCommands(%q) ranks %v first, want %s", tt.query, names(got), tt.first)
		}
	}
	if got := names(matchCommands("exit")); slices.Contains(got, "exit") {
		t.Errorf("matchCommands(exit) = %v; /exit is gone, /quit quits", got)
	}
	if got := matchCommands("xyz"); len(got) != 0 {
		t.Errorf("matchCommands(xyz) = %v, want nothing", names(got))
	}
}

func names(cs []command) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.name
	}
	return out
}

// msgOf runs cmd and returns its message, or nil.
func msgOf(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

func enter(m *Model) tea.Msg { return msgOf(m.Update(press(tea.KeyEnter))) }

// TestKillConfirms: /kill asks first; only Close sends the request.
func TestKillConfirms(t *testing.T) {
	m := screen(80, 24, session("api", time.Minute, 0), session("web", time.Hour, 0))
	m.Update(press(tea.KeyDown)) // api
	typeText(m, "/kill")
	if got := enter(m); got != nil {
		t.Fatalf("/kill sent %#v before confirming", got)
	}
	if m.confirm == nil {
		t.Fatal("/kill did not ask")
	}
	if !strings.Contains(ansi.Strip(m.View().Content), "Kill session api") {
		t.Fatal("the popup does not name the session")
	}
	if got := msgOf(m.Update(press(tea.KeyEscape))); got != nil || m.confirm != nil {
		t.Fatalf("Esc: sent %#v, popup open %v; want nothing sent and the popup closed", got, m.confirm != nil)
	}
	typeText(m, "/kill web")
	enter(m)
	got := msgOf(m.Update(tea.KeyPressMsg(tea.Key{Code: 'y', Text: "y"})))
	send, ok := got.(SendMsg)
	if k, isKill := send.Msg.(*api.KillSession); !ok || !isKill || k.Session != "web" {
		t.Fatalf("y sent %#v, want KillSession{web}", got)
	}
}

func TestSlashCommands(t *testing.T) {
	sessions := []api.SessionInfo{session("api", time.Minute, 0), session("web", time.Hour, 0)}
	tests := []struct {
		name   string
		pick   bool // highlight api before typing
		typed  string
		want   tea.Msg
		notice string // part of the notice, when the command is refused
	}{
		{"new with a name", false, "/new logs", OpenMsg{Session: "logs", Create: true}, ""},
		{"new without a name", false, "/new", OpenMsg{Create: true}, ""},
		{"new with an existing name attaches", false, "/new web", OpenMsg{Session: "web"}, ""},
		{"partial command runs the best match", false, "/ne", OpenMsg{Create: true}, ""},
		{"quit", false, "/quit", QuitMsg{}, ""},
		{"rename the highlighted session", true, "/rename backend", SendMsg{Msg: &api.RenameSession{Session: "api", Name: "backend"}}, ""},
		{"rename a named session", false, "/rename web front end", SendMsg{Msg: &api.RenameSession{Session: "web", Name: "front end"}}, ""},
		{"rename with nothing highlighted", false, "/rename backend", nil, "Select a session"},
		{"rename with no name asks for one", true, "/rename  ", nil, ""},
		{"rename with no session and no name", false, "/rename", nil, "Pick a session"},
		{"kill an unknown session", false, "/kill nope", nil, "No session named nope"},
		{"unknown command", false, "/xyz", nil, "Unknown command"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := screen(80, 24, sessions...)
			if tt.pick {
				m.Update(press(tea.KeyDown))
			}
			typeText(m, tt.typed)
			got := enter(m)
			if s, ok := got.(SendMsg); ok {
				w, _ := tt.want.(SendMsg)
				if r, ok := s.Msg.(*api.RenameSession); !ok || w.Msg == nil || *r != *w.Msg.(*api.RenameSession) {
					t.Fatalf("sent %#v, want %#v", s.Msg, w.Msg)
				}
				return
			}
			if got != tt.want {
				t.Fatalf("Enter on %q gave %#v, want %#v", tt.typed, got, tt.want)
			}
			if tt.notice != "" && !strings.Contains(m.notice, tt.notice) {
				t.Fatalf("notice %q, want it to say %q", m.notice, tt.notice)
			}
		})
	}
}

func TestTabCompletesTheCommand(t *testing.T) {
	m := screen(80, 24)
	typeText(m, "/ren")
	m.Update(press(tea.KeyTab))
	if m.input != "/rename " {
		t.Fatalf("input %q after Tab, want %q", m.input, "/rename ")
	}
	m = screen(80, 24)
	typeText(m, "/q")
	m.Update(press(tea.KeyDown))
	m.Update(press(tea.KeyTab))
	if m.input != "/quit" {
		t.Fatalf("Tab on a command with no argument: input %q, want /quit", m.input)
	}
}

func TestSessionsCommandShowsTheList(t *testing.T) {
	m := screen(80, 24, session("api", time.Minute, 0))
	typeText(m, "/sessions")
	if got := enter(m); got != nil {
		t.Fatalf("/sessions gave %#v", got)
	}
	if m.input != "" || m.selected != 0 {
		t.Fatalf("after /sessions: input %q, selected %d; want the list with the first session selected", m.input, m.selected)
	}
}

func TestHelpShowsCommandsAndKeys(t *testing.T) {
	m := screen(80, 24)
	typeText(m, "/help")
	enter(m)
	view := ansi.Strip(m.View().Content)
	for _, want := range []string{"/kill", "/rename", "Ctrl+b s", "Ctrl+b d"} {
		if !strings.Contains(view, want) {
			t.Errorf("help does not mention %s", want)
		}
	}
	m.Update(press(tea.KeyEscape))
	if m.help {
		t.Fatal("Esc did not close the help")
	}
}

// TestServerRefusalShows: a refused rename or kill shows the server's reason.
func TestServerRefusalShows(t *testing.T) {
	m := screen(80, 24)
	m.Update(&api.Error{Code: api.CodeCommandFailed, Message: "a session named web already exists"})
	if !strings.Contains(ansi.Strip(m.View().Content), "already exists") {
		t.Fatal("the refusal is not on screen")
	}
}

func TestCommandsGolden80x24(t *testing.T)  { goldenTyped(t, 80, 24, "/") }
func TestCommandsGolden120x40(t *testing.T) { goldenTyped(t, 120, 40, "/re") }
func TestHelpGolden80x24(t *testing.T)      { goldenTyped(t, 80, 24, "/help\r") }
func TestKillGolden80x24(t *testing.T)      { goldenTyped(t, 80, 24, "/kill api\r") }

// goldenTyped types typed on a home page with two sessions, where "\r" is
// Enter and "↓" is Down, and compares the view with its golden file.
func goldenTyped(t *testing.T, cols, rows int, typed string) {
	m := screen(cols, rows, session("api", time.Minute, 1), session("web", time.Hour, 0))
	for _, r := range typed {
		switch r {
		case '\r':
			m.Update(press(tea.KeyEnter))
			continue
		case '↓':
			m.Update(press(tea.KeyDown))
			continue
		}
		typeText(m, string(r))
	}
	golden.RequireEqual(t, m.View().Content)
}

// TestBadNamesAreRefused: a session name with a control character, or a
// long one, never reaches the server from the home page.
func TestBadNamesAreRefused(t *testing.T) {
	for _, typed := range []string{"/new " + strings.Repeat("a", api.MaxSessionName+1), strings.Repeat("b", api.MaxSessionName+1)} {
		m := screen(80, 24)
		typeText(m, typed)
		if got := enter(m); got != nil {
			t.Errorf("Enter on %.12q… gave %#v, want a refusal", typed, got)
		}
		if !strings.Contains(m.notice, "longer than") {
			t.Errorf("notice %q, want the reason", m.notice)
		}
	}
}

// TestBackFollowsRenameAndKill: Esc goes back to the session Ctrl+b s came
// from, under its new name after /rename, and quits once it is gone.
func TestBackFollowsRenameAndKill(t *testing.T) {
	m := screen(80, 24, session("api", time.Minute, 0))
	m.SetBack("api")
	typeText(m, "/rename api backend")
	enter(m)
	m.Update(&api.SessionList{Sessions: []api.SessionInfo{session("backend", time.Minute, 0)}})
	if got := msgOf(m.Update(press(tea.KeyEscape))); got != (OpenMsg{Session: "backend"}) {
		t.Fatalf("Esc after the rename gave %#v, want to open backend", got)
	}
	m.Update(&api.SessionList{}) // backend was killed
	if _, ok := msgOf(m.Update(press(tea.KeyEscape))).(QuitMsg); !ok {
		t.Fatal("Esc with the back session gone should quit")
	}
}

// TestSlashWithAltGr: on Windows, AltGr arrives as Ctrl+Alt with the text
// set. A "/" typed that way must still start a command (#84).
func TestSlashWithAltGr(t *testing.T) {
	m := screen(80, 24, session("0", time.Minute, 0))
	m.Update(tea.KeyPressMsg(tea.Key{Code: '/', Text: "/", Mod: tea.ModCtrl | tea.ModAlt}))
	typeText(m, "rename 0 test")
	got := enter(m)
	s, ok := got.(SendMsg)
	if !ok {
		t.Fatalf("Enter gave %#v, want a rename", got)
	}
	if r, ok := s.Msg.(*api.RenameSession); !ok || *r != (api.RenameSession{Session: "0", Name: "test"}) {
		t.Fatalf("sent %#v, want rename 0 to test", s.Msg)
	}
}

// TestCtrlAndAltShortcutsTypeNothing: a shortcut with only Ctrl or only Alt
// is not text, even when the terminal reports text for it.
func TestCtrlAndAltShortcutsTypeNothing(t *testing.T) {
	for _, k := range []tea.Key{
		{Code: 'a', Mod: tea.ModCtrl},
		{Code: 'x', Text: "x", Mod: tea.ModAlt},
		{Code: 'x', Text: "x", Mod: tea.ModCtrl},
	} {
		m := screen(80, 24)
		m.Update(tea.KeyPressMsg(k))
		if m.input != "" {
			t.Errorf("%v typed %q, want nothing", k, m.input)
		}
	}
}

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
