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

func TestEndAndLostConnectionDiffer(t *testing.T) {
	newApp := func() *app {
		send := func(api.Message) {}
		return &app{screen: session.New(send), send: send}
	}
	ended := newApp()
	ended.Update(&api.Error{Code: api.CodeSessionEnded, Message: "session work ended"})
	ended.Update(connClosedMsg{})
	if !ended.result.Ended || ended.result.Err != nil {
		t.Fatalf("after session-ended: %+v, want Ended and no error", ended.result)
	}

	lost := newApp()
	lost.Update(connClosedMsg{})
	if lost.result.Ended || !errors.Is(lost.result.Err, ErrConnectionLost) {
		t.Fatalf("after a bare close: %+v, want ErrConnectionLost", lost.result)
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

// TestHomeKeyKeepsSession: Ctrl+b s shows the home page without ending the
// session, and Esc goes back to it.
func TestHomeKeyKeepsSession(t *testing.T) {
	var sent []api.Message
	a := newHomeApp(func(m api.Message) { sent = append(sent, m) }, "v0.1.0", "", theme.Default())
	openSession(t, a, "api")
	a.Update(session.SwitchMsg{})
	if _, ok := sent[len(sent)-1].(*api.Detach); !ok {
		t.Fatalf("sent %#v, want Detach", sent[len(sent)-1])
	}
	if _, ok := a.screen.(*home.Model); !ok {
		t.Fatalf("screen is %T after Ctrl+b s, want the home page", a.screen)
	}
	_, cmd := a.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	run(a, cmd)
	at, ok := sent[len(sent)-1].(*api.Attach)
	if !ok || at.Session != "api" || at.Create {
		t.Fatalf("sent %#v, want Attach{api} back to the session", sent[len(sent)-1])
	}
	if _, ok := a.screen.(*session.Model); !ok {
		t.Fatalf("screen is %T after Esc, want the session screen", a.screen)
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

// TestSwitchKeepsBackWithAnOldList: the list the app saw before attaching
// does not have the session created since; Ctrl+b s must still go back.
func TestSwitchKeepsBackWithAnOldList(t *testing.T) {
	var sent []api.Message
	a := newHomeApp(func(m api.Message) { sent = append(sent, m) }, "v0.1.0", "", theme.Default())
	a.Update(&api.SessionList{}) // before "api" existed
	openSession(t, a, "api")
	a.Update(session.SwitchMsg{})
	_, cmd := a.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	run(a, cmd)
	if at, ok := sent[len(sent)-1].(*api.Attach); !ok || at.Session != "api" {
		t.Fatalf("sent %#v, want Attach{api}: Esc lost the way back", sent[len(sent)-1])
	}
}
