package home

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
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
		{"rename to a bad name", true, "/rename  ", nil, "/rename"},
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

func goldenTyped(t *testing.T, cols, rows int, typed string) {
	m := screen(cols, rows, session("api", time.Minute, 1), session("web", time.Hour, 0))
	for _, r := range typed {
		if r == '\r' {
			m.Update(press(tea.KeyEnter))
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
