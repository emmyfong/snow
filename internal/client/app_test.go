package client

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/emmyfong/snow/internal/client/home"
	"github.com/emmyfong/snow/internal/client/keys"
	"github.com/emmyfong/snow/internal/client/session"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/config"
	"github.com/emmyfong/snow/pkg/api"
)

func TestWindowTitleNamesTheSession(t *testing.T) {
	send := func(api.Message) {}
	a := &app{screen: session.New(send), send: send}
	if got := a.View().WindowTitle; got != "snow" {
		t.Fatalf("title before attaching = %q, want snow", got)
	}
	a.Update(&api.Layout{Session: "work", Active: 1, Windows: []api.WindowInfo{{Index: 1, Panes: []api.PaneInfo{{ID: 1, Rect: api.Rect{W: 80, H: 24}}}}}})
	if got := a.View().WindowTitle; got != "snow: work" {
		t.Fatalf("title = %q, want %q", got, "snow: work")
	}
}

func TestHugeTerminalSendsAcceptedSize(t *testing.T) {
	var sent []api.Message
	send := func(m api.Message) { sent = append(sent, m) }
	a := &app{screen: session.New(send), send: send}
	a.Update(tea.WindowSizeMsg{Width: 5000, Height: 2000})
	r, ok := sent[len(sent)-1].(*api.Resize)
	if !ok {
		t.Fatalf("sent %T, want *api.Resize", sent[len(sent)-1])
	}
	if err := r.Validate(); err != nil {
		t.Fatalf("client sent a size the server rejects: %v", err)
	}
}

// TestUnknownSizeNotSent: without a terminal, Bubble Tea reports 0x0. That
// must not shrink a shared session to one cell.
func TestUnknownSizeNotSent(t *testing.T) {
	var sent []api.Message
	send := func(m api.Message) { sent = append(sent, m) }
	a := &app{screen: session.New(send), send: send}
	a.Update(tea.WindowSizeMsg{Width: 0, Height: 0})
	if len(sent) != 0 {
		t.Fatalf("sent %v for an unknown size, want nothing", sent)
	}
}

// TestSessionEndShowsHome: when the attached session ends, as when its last
// pane exits, the client shows the home page with a note instead of quitting.
func TestSessionEndShowsHome(t *testing.T) {
	send := func(api.Message) {}
	a := &app{screen: session.New(send), send: send}
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	a.Update(&api.Layout{Session: "work", Active: 1, Windows: []api.WindowInfo{{Index: 1, Panes: []api.PaneInfo{{ID: 1, Rect: api.Rect{W: 80, H: 24}}}}}})
	_, cmd := a.Update(&api.Error{Code: api.CodeSessionEnded, Message: "session work ended"})
	if cmd != nil || a.result.Err != nil {
		t.Fatalf("the session end quit Snow: result %+v", a.result)
	}
	if _, ok := a.screen.(*home.Model); !ok {
		t.Fatalf("screen is %T, want the home page", a.screen)
	}
	if !strings.Contains(a.View().Content, "session work ended") {
		t.Fatal("the home page should say the session ended")
	}
	if a.result.Session != "" || a.View().WindowTitle != "snow" {
		t.Fatalf("session %q, title %q: the ended session should be forgotten", a.result.Session, a.View().WindowTitle)
	}
}

// TestLateSessionEndIgnored: an end that arrives after the user left the
// session, for the home page or another session, leaves that screen alone.
func TestLateSessionEndIgnored(t *testing.T) {
	ended := &api.Error{Code: api.CodeSessionEnded, Message: "session work ended"}
	layout := &api.Layout{Session: "work", Active: 1, Windows: []api.WindowInfo{{Index: 1, Panes: []api.PaneInfo{{ID: 1, Rect: api.Rect{W: 80, H: 24}}}}}}

	home1 := newHomeApp(func(api.Message) {}, "v0.1.0", "", theme.Default())
	home1.Update(layout)
	home1.Update(session.DetachMsg{})
	before := home1.screen
	home1.Update(ended)
	if home1.screen != before || !home1.result.Detached {
		t.Fatal("a late end replaced the home page the user detached to")
	}

	opening := newHomeApp(func(api.Message) {}, "v0.1.0", "", theme.Default())
	opening.Update(layout)
	opening.Update(session.SwitchMsg{Session: "api"})
	before = opening.screen
	opening.Update(ended)
	if opening.screen != before || !opening.opening {
		t.Fatal("a late end replaced the session the user is opening")
	}
}

func TestLostConnectionQuits(t *testing.T) {
	send := func(api.Message) {}
	lost := &app{screen: session.New(send), send: send}
	if _, cmd := lost.Update(connClosedMsg{}); cmd == nil || !errors.Is(lost.result.Err, ErrConnectionLost) {
		t.Fatalf("after a bare close: %+v, want ErrConnectionLost and a quit", lost.result)
	}
}

// TestCommandErrorKeepsClient: a refused command, such as a split with no
// room, must not end the client.
func TestCommandErrorKeepsClient(t *testing.T) {
	send := func(api.Message) {}
	for _, code := range []string{api.CodeCommandFailed, api.CodeBadCommand, api.CodeBadSize} {
		a := &app{screen: session.New(send), send: send}
		_, cmd := a.Update(&api.Error{Code: code, Message: "no room"})
		if cmd != nil && a.result.Err != nil {
			t.Fatalf("error %s ended the client: %v", code, a.result.Err)
		}
	}
	a := &app{screen: session.New(send), send: send}
	if _, cmd := a.Update(&api.Error{Code: api.CodeNoSession}); cmd == nil || a.result.Err == nil {
		t.Fatal("an attach error should end the client")
	}
}

func TestRunRejectsUnknownTheme(t *testing.T) {
	cfg := config.Defaults()
	cfg.Theme.Name = "neon"
	_, err := Run(Options{Config: &cfg})
	if !errors.Is(err, theme.ErrUnknownTheme) {
		t.Fatalf("Run error = %v, want ErrUnknownTheme", err)
	}
}

func TestRunRejectsBadKeyBinding(t *testing.T) {
	cfg := config.Defaults()
	cfg.Keys.Bind = map[string]string{"|": "split-sideways"}
	_, err := Run(Options{Config: &cfg})
	if !errors.Is(err, keys.ErrUnknownAction) {
		t.Fatalf("Run error = %v, want ErrUnknownAction", err)
	}
}

// TestHomeOpensASession: the home page's choice attaches, and the client
// switches to the session screen.
func TestHomeOpensASession(t *testing.T) {
	var sent []api.Message
	send := func(m api.Message) { sent = append(sent, m) }
	a := newHomeApp(send, "v0.1.0", "/src", theme.Default())
	if a.View().WindowTitle != "snow" {
		t.Fatalf("home title %q, want snow", a.View().WindowTitle)
	}
	a.Update(home.OpenMsg{Session: "api", Create: true})
	at, ok := sent[len(sent)-1].(*api.Attach)
	if !ok || at.Session != "api" || !at.Create || at.Dir != "/src" {
		t.Fatalf("sent %#v, want Attach{api, create, /src}", sent[len(sent)-1])
	}
	if _, ok := a.screen.(*session.Model); !ok {
		t.Fatalf("screen is %T after opening, want the session screen", a.screen)
	}
}

func TestHomeQuitEndsWithoutASession(t *testing.T) {
	a := newHomeApp(func(api.Message) {}, "v0.1.0", "", theme.Default())
	_, cmd := a.Update(home.QuitMsg{})
	if cmd == nil || a.result.Session != "" || a.result.Err != nil {
		t.Fatalf("quit from home: cmd %v, result %+v", cmd, a.result)
	}
}

// openSession takes a home app into session name, as Enter on the home
// page does.
func openSession(t *testing.T, a *app, name string) {
	t.Helper()
	a.Update(home.OpenMsg{Session: name, Create: true})
	a.Update(&api.Layout{Session: name, Active: 0, Windows: []api.WindowInfo{{Index: 0, Panes: []api.PaneInfo{{ID: 1, Focused: true, Rect: api.Rect{W: 80, H: 23}}}}}})
	if _, ok := a.screen.(*session.Model); !ok {
		t.Fatalf("screen is %T, want the session screen", a.screen)
	}
}

// run feeds the message a command produces back into a, as Bubble Tea does.
func run(a *app, cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	_, next := a.Update(cmd())
	return next
}

// TestDetachShowsHome: Ctrl+b d leaves the session running and shows the
// home page; Esc there quits.
func TestDetachShowsHome(t *testing.T) {
	var sent []api.Message
	a := newHomeApp(func(m api.Message) { sent = append(sent, m) }, "v0.1.0", "", theme.Default())
	openSession(t, a, "api")
	a.Update(session.DetachMsg{})
	if _, ok := sent[len(sent)-1].(*api.Detach); !ok {
		t.Fatalf("sent %#v, want Detach", sent[len(sent)-1])
	}
	if _, ok := a.screen.(*home.Model); !ok {
		t.Fatalf("screen is %T after detach, want the home page", a.screen)
	}
	_, cmd := a.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if _, ok := cmd().(home.QuitMsg); !ok {
		t.Fatal("Esc on the home page after detach should quit")
	}
	if !a.result.Detached || a.result.Session != "api" {
		t.Fatalf("result %+v, want detached from api", a.result)
	}
}

// TestNewScreensGetTheColorProfile: the terminal reports its colors once;
// a screen opened later must still adapt to them.
func TestNewScreensGetTheColorProfile(t *testing.T) {
	a := newHomeApp(func(api.Message) {}, "v0.1.0", "", theme.Default())
	a.Update(tea.ColorProfileMsg{Profile: colorprofile.ANSI})
	if a.profile == nil || a.profile.Profile != colorprofile.ANSI {
		t.Fatal("the app did not keep the color profile for later screens")
	}
}

// TestHomeRequestsReachTheServer: the home page asks the app to send a kill
// or a rename; the app sends it as is.
func TestHomeRequestsReachTheServer(t *testing.T) {
	var sent []api.Message
	a := newHomeApp(func(m api.Message) { sent = append(sent, m) }, "v0.1.0", "", theme.Default())
	kill := &api.KillSession{Session: "api"}
	a.Update(home.SendMsg{Msg: kill})
	if len(sent) != 1 || sent[0] != kill {
		t.Fatalf("sent %v, want the kill request", sent)
	}
}

// TestRefusedOpenReturnsHome: opening a session that just ended (or the
// back session after it was killed) returns to the home page with the
// reason, instead of quitting Snow.
func TestRefusedOpenReturnsHome(t *testing.T) {
	a := newHomeApp(func(api.Message) {}, "v0.1.0", "", theme.Default())
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	a.Update(&api.SessionList{Sessions: []api.SessionInfo{{Name: "api"}}})
	a.Update(home.OpenMsg{Session: "api"})
	_, cmd := a.Update(&api.Error{Code: api.CodeNoSession, Message: "no session named api"})
	if cmd != nil || a.result.Err != nil {
		t.Fatalf("a refused open quit Snow: result %+v", a.result)
	}
	if _, ok := a.screen.(*home.Model); !ok {
		t.Fatalf("screen is %T, want the home page", a.screen)
	}
	view := a.View().Content
	if !strings.Contains(view, "no session named api") || !strings.Contains(view, "api") {
		t.Fatal("the home page should show the reason and the last session list")
	}
}

// TestThemeIsAppliedAndSaved: a theme picked on the home page is used by
// screens opened after it and saved; a failed save says so on the page.
func TestThemeIsAppliedAndSaved(t *testing.T) {
	a := newHomeApp(func(api.Message) {}, "v0.1.0", "", theme.Default())
	var saved []string
	a.saveTheme = func(name string) error { saved = append(saved, name); return nil }
	_, cmd := a.Update(home.ThemeMsg{Name: "Nord"})
	run(a, cmd)
	if a.theme.Name != "Nord" || len(saved) != 1 || saved[0] != "Nord" {
		t.Fatalf("theme %q, saved %v; want Nord used and saved", a.theme.Name, saved)
	}
	a.saveTheme = func(string) error { return errors.New("disk full") }
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	_, cmd = a.Update(home.ThemeMsg{Name: "Dracula"})
	run(a, cmd)
	if !strings.Contains(a.View().Content, "disk full") {
		t.Fatal("a failed save is not on the home page")
	}
}

// TestSwitchAttachesOtherSession: a session chosen in the session list
// detaches from this one, which keeps running, and attaches to the other.
func TestSwitchAttachesOtherSession(t *testing.T) {
	var sent []api.Message
	a := newHomeApp(func(m api.Message) { sent = append(sent, m) }, "v0.1.0", "/src", theme.Default())
	openSession(t, a, "api")
	old := a.screen
	n := len(sent)
	a.Update(session.SwitchMsg{Session: "web"})
	if len(sent) != n+2 {
		t.Fatalf("sent %v, want Detach then Attach", sent[n:])
	}
	if _, ok := sent[n].(*api.Detach); !ok {
		t.Fatalf("first sent %#v, want Detach", sent[n])
	}
	at, ok := sent[n+1].(*api.Attach)
	if !ok || at.Session != "web" || at.Create || at.Dir != "/src" {
		t.Fatalf("then sent %#v, want Attach{web, /src}", sent[n+1])
	}
	if _, ok := a.screen.(*session.Model); !ok || a.screen == old || !a.opening || a.result.Detached {
		t.Fatalf("screen %T (new %v), opening %v, detached %v; want a new session screen opening web", a.screen, a.screen != old, a.opening, a.result.Detached)
	}
	a.Update(&api.Layout{Session: "web", Windows: []api.WindowInfo{{Panes: []api.PaneInfo{{ID: 4, Focused: true, Rect: api.Rect{W: 80, H: 23}}}}}})
	if a.View().WindowTitle != "snow: web" {
		t.Fatalf("title %q, want snow: web", a.View().WindowTitle)
	}
}

// TestSwitchRefusedShowsHome: a session that ended between the list and the
// choice leaves the client detached, on the home page with the reason.
func TestSwitchRefusedShowsHome(t *testing.T) {
	a := newHomeApp(func(api.Message) {}, "v0.1.0", "", theme.Default())
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	openSession(t, a, "api")
	a.Update(session.SwitchMsg{Session: "web"})
	_, cmd := a.Update(&api.Error{Code: api.CodeNoSession, Message: "no session named web"})
	if cmd != nil || a.result.Err != nil {
		t.Fatalf("a refused switch quit Snow: result %+v", a.result)
	}
	if _, ok := a.screen.(*home.Model); !ok {
		t.Fatalf("screen is %T, want the home page", a.screen)
	}
	if !strings.Contains(a.View().Content, "no session named web") {
		t.Fatal("the home page should say why")
	}
}

// TestSessionListReachesTheSessionScreen: the list the server sends an
// attached client reaches the session screen, which shows it in the list.
func TestSessionListReachesTheSessionScreen(t *testing.T) {
	a := newHomeApp(func(api.Message) {}, "v0.1.0", "", theme.Default())
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	openSession(t, a, "api")
	a.Update(&api.SessionList{Sessions: []api.SessionInfo{{Name: "api", State: api.StateRunning}, {Name: "web", State: api.StateRunning}}})
	a.Update(tea.KeyPressMsg{Code: 'b', Mod: tea.ModCtrl})
	a.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	if !strings.Contains(a.View().Content, "web") {
		t.Fatal("the session list does not show web")
	}
}
