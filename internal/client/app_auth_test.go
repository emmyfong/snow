package client

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/emmyfong/snow/internal/client/session"
	"github.com/emmyfong/snow/pkg/api"
)

// authApp is an app on a session screen with one focused chat pane. It
// records what the app sends.
type authApp struct {
	*app
	sent []api.Message
}

func newAuthApp() *authApp {
	a := &authApp{}
	send := func(m api.Message) { a.sent = append(a.sent, m) }
	a.app = &app{screen: session.New(send), send: send}
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	a.Update(&api.Layout{Session: "work", Active: 1, Windows: []api.WindowInfo{{Index: 1, Name: "sh", Panes: []api.PaneInfo{
		{ID: 2, Kind: api.PaneChat, Rect: api.Rect{W: 100, H: 29}, Focused: true},
	}}}})
	a.Update(&api.ChatSnapshot{Pane: 2, Agent: "c1", Conversation: "c1", Model: "anthropic/claude-sonnet-5-5", Mode: "ask", State: api.ChatStateIdle})
	return a
}

// typeKeys types text, then presses Enter when enter is set.
func (a *authApp) typeKeys(text string, enter bool) {
	for _, r := range text {
		a.Update(tea.KeyPressMsg(tea.Key{Code: r, Text: string(r)}))
	}
	if enter {
		a.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	}
}

func (a *authApp) screenText() string { return ansi.Strip(a.View().Content) }

// sentLast reports whether the last message sent has the type of want.
func sentLast[T api.Message](a *authApp) bool {
	if len(a.sent) == 0 {
		return false
	}
	_, ok := a.sent[len(a.sent)-1].(T)
	return ok
}

// startLogin opens /login, submits a key for Anthropic, and closes the
// picker during the check when closePicker is set.
func startLogin(t *testing.T, a *authApp, closePicker bool) {
	t.Helper()
	a.typeKeys("/login", true)
	a.Update(&api.AuthList{Providers: []api.ProviderAuth{{Name: "anthropic", Title: "Anthropic"}}})
	if !strings.Contains(a.screenText(), "Log in") {
		t.Fatalf("/login did not open the picker:\n%s", a.screenText())
	}
	a.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	a.typeKeys("sk-bad", true)
	if !sentLast[*api.LoginRequest](a) {
		t.Fatalf("the picker did not send the key; sent %v", a.sent)
	}
	if closePicker {
		a.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
		if strings.Contains(a.screenText(), "Log in") {
			t.Fatal("Escape did not close the picker during the check")
		}
	}
}

// startLogout runs /logout for the saved Anthropic key and confirms it.
func startLogout(t *testing.T, a *authApp) {
	t.Helper()
	a.typeKeys("/logout", true)
	a.Update(&api.AuthList{Providers: []api.ProviderAuth{{Name: "anthropic", Title: "Anthropic",
		Source: "OS credential store", Connected: true}}})
	a.typeKeys("y", false)
	if !sentLast[*api.LogoutRequest](a) {
		t.Fatalf("/logout did not send the request; sent %v", a.sent)
	}
}

// TestAuthFailedKeepsClient: a refused login or logout keeps the client
// running (#237). A rejected key shows in the login picker, which returns
// to the key field, or in the status bar when the picker closed during the
// check.
func TestAuthFailedKeepsClient(t *testing.T) {
	const rejected = "Anthropic (Claude) did not accept the key: 401"
	for _, tc := range []struct {
		name    string
		setup   func(*testing.T, *authApp)
		reason  string
		want    []string
		notWant []string
	}{
		{
			name:    "login, picker open",
			setup:   func(t *testing.T, a *authApp) { startLogin(t, a, false) },
			reason:  rejected,
			want:    []string{"did not accept the key", "Log in · Anthropic"},
			notWant: []string{"Checking the key"},
		},
		{
			name:    "login, picker closed",
			setup:   func(t *testing.T, a *authApp) { startLogin(t, a, true) },
			reason:  rejected,
			want:    []string{"did not accept the key"},
			notWant: []string{"Log in"},
		},
		{
			name:   "logout",
			setup:  startLogout,
			reason: "the key store did not answer",
			want:   []string{"the key store did not answer"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newAuthApp()
			tc.setup(t, a)
			a.Update(&api.Error{Code: api.CodeAuthFailed, Message: tc.reason})
			if a.result.Err != nil {
				t.Fatalf("a refused request ended the client: %v", a.result.Err)
			}
			s := a.screenText()
			for _, w := range tc.want {
				if !strings.Contains(s, w) {
					t.Errorf("the screen does not show %q:\n%s", w, s)
				}
			}
			for _, w := range tc.notWant {
				if strings.Contains(s, w) {
					t.Errorf("the screen still shows %q:\n%s", w, s)
				}
			}
		})
	}
}
