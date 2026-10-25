package session

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/emmyfong/snow/internal/client/command"
	"github.com/emmyfong/snow/pkg/api"
)

// openPrompt presses Ctrl+b : on the session "api" that knows the list.
func openPrompt(t *testing.T, cols, rows int) (*Model, *recorder) {
	t.Helper()
	m, r := switchScreen(t, cols, rows)
	m.Update(ctrlB)
	m.Update(press(':', 0, ":"))
	return m, r
}

// typeIn types text into the screen; "\r" presses Enter. It returns the
// command the last key gave.
func typeIn(m *Model, text string) tea.Cmd {
	var cmd tea.Cmd
	for _, r := range text {
		switch r {
		case '\r':
			cmd = m.Update(enter)
		case ' ':
			cmd = m.Update(press(tea.KeySpace, 0, " "))
		default:
			cmd = m.Update(press(r, 0, string(r)))
		}
	}
	return cmd
}

func lastSent(r *recorder) api.Message {
	if len(r.sent) == 0 {
		return nil
	}
	return r.sent[len(r.sent)-1]
}

func promptOpen(m *Model) bool { return strings.Contains(plain(m), "Commands") }

func TestCommandPromptOpensPopup(t *testing.T) {
	m, r := openPrompt(t, 80, 24)
	view := plain(m)
	for _, want := range []string{"Commands", "rename [name]", "kill [session]", "new [name]", "switch [session]", "theme [name]"} {
		if !strings.Contains(view, want) {
			t.Errorf("the popup does not show %q:\n%s", want, view)
		}
	}
	if len(r.sent) != 0 {
		t.Fatalf("sent %v, want nothing", r.sent)
	}
}

// TestCommandPopupHidesHomeOnly: help and quit belong to the home page.
func TestCommandPopupHidesHomeOnly(t *testing.T) {
	m, _ := openPrompt(t, 80, 24)
	for _, name := range []string{"quit", "help", "sessions"} {
		if strings.Contains(plain(m), name) {
			t.Errorf("the session popup lists the home command %s", name)
		}
	}
	typeIn(m, "quit\r")
	if !strings.Contains(plain(m), "Unknown command quit") {
		t.Fatal("quit ran in a session")
	}
}

func TestCommandPopupRenameCurrent(t *testing.T) {
	m, r := openPrompt(t, 80, 24)
	typeIn(m, "rename backend\r")
	if got, ok := lastSent(r).(*api.RenameSession); !ok || *got != (api.RenameSession{Session: "api", Name: "backend"}) {
		t.Fatalf("sent %#v, want rename api to backend", lastSent(r))
	}
	if promptOpen(m) {
		t.Fatal("the popup stays open after the rename")
	}
}

func TestCommandPopupRenameAsksName(t *testing.T) {
	m, r := openPrompt(t, 80, 24)
	typeIn(m, "rename\r")
	if !strings.Contains(plain(m), "New name for api:") || len(r.sent) != 0 {
		t.Fatalf("rename with no name sent %v; want the name prompt:\n%s", r.sent, plain(m))
	}
	m.Update(press('u', tea.ModCtrl, ""))
	typeIn(m, "\r")
	if !strings.Contains(plain(m), "the name is empty") || len(r.sent) != 0 {
		t.Fatalf("an empty name sent %v; want the reason shown", r.sent)
	}
	typeIn(m, "web2\r")
	if got, ok := lastSent(r).(*api.RenameSession); !ok || *got != (api.RenameSession{Session: "api", Name: "web2"}) {
		t.Fatalf("sent %#v, want rename api to web2", lastSent(r))
	}
}

func TestCommandPopupKillConfirms(t *testing.T) {
	m, r := openPrompt(t, 80, 24)
	typeIn(m, "kill\r")
	if !strings.Contains(plain(m), "Kill session api and its programs?") || len(r.sent) != 0 {
		t.Fatalf("kill sent %v before the question:\n%s", r.sent, plain(m))
	}
	m.Update(press('n', 0, "n"))
	if len(r.sent) != 0 || m.confirm != nil {
		t.Fatalf("n sent %v; want nothing and the question closed", r.sent)
	}

	m, r = openPrompt(t, 80, 24)
	typeIn(m, "kill web\r")
	m.Update(press('y', 0, "y"))
	if got, ok := lastSent(r).(*api.KillSession); !ok || got.Session != "web" {
		t.Fatalf("sent %#v, want kill web", lastSent(r))
	}

	m, r = openPrompt(t, 80, 24)
	typeIn(m, "kill nope\r")
	if !strings.Contains(plain(m), "No session named nope.") || len(r.sent) != 0 {
		t.Fatal("kill of an unknown session should say so and send nothing")
	}
}

func TestCommandPopupNewSwitches(t *testing.T) {
	for _, tt := range []struct {
		typed string
		want  SwitchMsg
	}{
		{"new docs\r", SwitchMsg{Session: "docs", Create: true}},
		{"new web\r", SwitchMsg{Session: "web"}},
		{"new\r", SwitchMsg{Create: true}},
	} {
		m, r := openPrompt(t, 80, 24)
		s, ok := switchOf(typeIn(m, tt.typed))
		if !ok || s != tt.want || len(r.sent) != 0 {
			t.Errorf("%q gave %#v and sent %v, want %#v", tt.typed, s, r.sent, tt.want)
		}
	}
	m, _ := openPrompt(t, 80, 24)
	if s, ok := switchOf(typeIn(m, "new api\r")); ok {
		t.Errorf("new api, the current session, switched: %#v", s)
	}
}

func TestCommandPopupSwitchOpensList(t *testing.T) {
	m, _ := openPrompt(t, 80, 24)
	typeIn(m, "switch\r")
	if m.switcher == nil || promptOpen(m) {
		t.Fatal("switch with no name should open the session list")
	}
	m, _ = openPrompt(t, 80, 24)
	if s, ok := switchOf(typeIn(m, "switch web\r")); !ok || s != (SwitchMsg{Session: "web"}) {
		t.Fatalf("switch web gave %#v", s)
	}
	m, _ = openPrompt(t, 80, 24)
	typeIn(m, "switch nope\r")
	if !strings.Contains(plain(m), "No session named nope.") {
		t.Fatal("switch to an unknown session should say so")
	}
}

func themeOf(cmd tea.Cmd) (command.ThemeMsg, bool) {
	if cmd == nil {
		return command.ThemeMsg{}, false
	}
	th, ok := cmd().(command.ThemeMsg)
	return th, ok
}

func TestCommandPopupThemeApplies(t *testing.T) {
	m, _ := openPrompt(t, 80, 24)
	if th, ok := themeOf(typeIn(m, "theme dracula\r")); !ok || th.Name != "Dracula" {
		t.Fatalf("theme dracula gave %#v", th)
	}
	m, _ = openPrompt(t, 80, 24)
	typeIn(m, "theme neon\r")
	if !strings.Contains(plain(m), "neon") || !promptOpen(m) {
		t.Fatal("an unknown theme should say so and keep the popup")
	}
	m, _ = openPrompt(t, 80, 24)
	typeIn(m, "theme\r")
	if !strings.Contains(plain(m), "Themes") || !strings.Contains(plain(m), "Nord") {
		t.Fatalf("theme with no name should list the themes:\n%s", plain(m))
	}
	if th, ok := themeOf(typeIn(m, "nord\r")); !ok || th.Name != "Nord" {
		t.Fatalf("picking nord gave %#v", th)
	}
	if strings.Contains(plain(m), "Themes") {
		t.Fatal("the theme list stays open")
	}
}

func TestCommandPopupUnknownCommand(t *testing.T) {
	m, r := openPrompt(t, 80, 24)
	typeIn(m, "zzz\r")
	if !strings.Contains(plain(m), "Unknown command zzz") || len(r.sent) != 0 {
		t.Fatalf("unknown command sent %v:\n%s", r.sent, plain(m))
	}
}

func TestCommandPopupTabCompletes(t *testing.T) {
	m, _ := openPrompt(t, 80, 24)
	typeIn(m, "re")
	m.Update(press(tea.KeyTab, 0, ""))
	if m.prompt == nil {
		t.Fatal("Tab closed the popup")
	}
	if m.prompt.input != "rename " {
		t.Fatalf("Tab after re gave %q, want \"rename \"", m.prompt.input)
	}
}

func TestCommandPopupEscCloses(t *testing.T) {
	m, r := openPrompt(t, 80, 24)
	m.Update(press(tea.KeyEscape, 0, ""))
	m.Update(press('l', 0, "l"))
	if promptOpen(m) || len(r.sent) != 1 || r.keys()[0] != "l" {
		t.Fatalf("after Esc: open %v, sent %v; want closed and only l sent", promptOpen(m), r.sent)
	}
}

// TestCommandPopupClickRuns: a click on a command runs it with its default
// argument, so the mouse reaches every command.
func TestCommandPopupClickRuns(t *testing.T) {
	m, _ := openPrompt(t, 80, 24)
	for y, row := range strings.Split(plain(m), "\n") {
		if x := strings.Index(row, "switch [session]"); x >= 0 {
			m.Update(click(x, y))
			if m.switcher == nil {
				t.Fatal("a click on switch did not open the session list")
			}
			return
		}
	}
	t.Fatal("switch is not drawn")
}

func TestMenuOpensCommandPrompt(t *testing.T) {
	m, _ := switchScreen(t, 80, 24)
	chooseByClick(t, m, "Command prompt")
	if !promptOpen(m) {
		t.Fatal("Command prompt in the menu did not open the popup")
	}
}

// TestEverySessionCommandHasHandler: each command the registry lists for a
// session does something here.
func TestEverySessionCommandHasHandler(t *testing.T) {
	for _, c := range command.In(command.InSession) {
		if commandRuns[c.Name] == nil {
			t.Errorf("no handler for %s", c.Name)
		}
	}
}

func goldenPrompt(t *testing.T, cols, rows int, typed string) {
	m, _ := openPrompt(t, cols, rows)
	typeIn(m, typed)
	golden.RequireEqual(t, m.View().Content)
}

func TestCommandPopupGolden80x24(t *testing.T)      { goldenPrompt(t, 80, 24, "") }
func TestCommandPopupGolden120x40(t *testing.T)     { goldenPrompt(t, 120, 40, "") }
func TestCommandPopupTypedGolden80x24(t *testing.T) { goldenPrompt(t, 80, 24, "re") }
func TestCommandThemesGolden80x24(t *testing.T)     { goldenPrompt(t, 80, 24, "theme\r") }
func TestCommandRenameGolden80x24(t *testing.T)     { goldenPrompt(t, 80, 24, "rename\r") }

// TestCommandPopupTinyScreen: a window too small for the popup does not
// open it, says why, and leaves the keys to the pane; a small window that
// fits still runs commands.
func TestCommandPopupTinyScreen(t *testing.T) {
	for _, sz := range [][2]int{{10, 5}, {23, 24}, {80, 7}} {
		m, r := openPrompt(t, sz[0], sz[1])
		if m.prompt != nil || m.message != "Window too small for the command list" {
			t.Errorf("%dx%d: prompt open %v, message %q", sz[0], sz[1], m.prompt != nil, m.message)
		}
		m.Update(press('l', 0, "l"))
		if got := r.keys(); len(got) != 1 || got[0] != "l" {
			t.Errorf("%dx%d: sent %v, want the key l for the pane", sz[0], sz[1], r.sent)
		}
	}
	for _, sz := range [][2]int{{30, 8}, {40, 12}} {
		m, _ := openPrompt(t, sz[0], sz[1])
		typeIn(m, "theme\r")
		_ = m.View()
		m.Update(click(1, 1))
		m, r := openPrompt(t, sz[0], sz[1])
		typeIn(m, "rename x\r")
		if got, ok := lastSent(r).(*api.RenameSession); !ok || got.Name != "x" {
			t.Errorf("%dx%d: rename x sent %#v", sz[0], sz[1], lastSent(r))
		}
	}
}
