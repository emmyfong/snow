package client

import (
	"errors"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/home"
	"github.com/emmyfong/snow/internal/client/session"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// Result says how a client run ended.
type Result struct {
	Session  string // the attached session's name
	Detached bool   // the user detached; the session keeps running
	Ended    bool   // the session ended, for example because its shell exited
	Err      error  // a refused request (*api.Error) or ErrConnectionLost
}

// ErrConnectionLost reports that the connection closed without the server
// saying why: the server crashed or was killed, or dropped this client.
var ErrConnectionLost = errors.New("lost connection to the server")

// connClosedMsg reports that the server closed the connection.
type connClosedMsg struct{}

// app is the root model. It routes messages to the current screen and ends
// the program on detach, on a server error, or when the connection closes.
type app struct {
	screen     Screen
	send       func(api.Message)
	result     Result
	dir        string           // the folder a session opened from the home page starts in
	version    string           // Snow's version, for the home page
	theme      theme.Theme      // the theme from the config, for the home page
	sessOpts   []session.Option // the theme and keys a session screen gets
	profile    *tea.ColorProfileMsg
	cols, rows int
}

// newHomeApp returns an app that starts on the home page. Opening a session
// there attaches and switches to the session screen.
func newHomeApp(send func(api.Message), version, dir string, th theme.Theme, opts ...session.Option) *app {
	return &app{screen: home.New(version, th), send: send, dir: dir, version: version, theme: th, sessOpts: opts}
}

// newSession returns a session screen with the app's theme and keys.
func (a *app) newSession() *session.Model { return session.New(a.send, a.sessOpts...) }

// show makes s the current screen, at the terminal's size and colors: the
// terminal reports its colors once, at start, so a later screen needs them
// passed on.
func (a *app) show(s Screen) {
	s.SetSize(a.cols, a.rows)
	if a.profile != nil {
		s.Update(*a.profile)
	}
	a.screen = s
}

// leave detaches from the session, which keeps running, and shows the home
// page. With back set, Esc there returns to that session.
func (a *app) leave(back string) {
	a.send(&api.Detach{})
	h := home.New(a.version, a.theme)
	h.SetBack(back)
	a.show(h)
}

func (a *app) Init() tea.Cmd { return nil }

func (a *app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.cols, a.rows = msg.Width, msg.Height
		a.screen.SetSize(msg.Width, msg.Height)
		if cols, rows, ok := acceptedSize(msg.Width, msg.Height); ok {
			a.send(&api.Resize{Cols: cols, Rows: rows})
		}
		return a, nil
	case tea.ColorProfileMsg:
		a.profile = &msg
	case *api.Layout:
		a.result.Session = msg.Session
	case *api.Error:
		switch msg.Code {
		case api.CodeSessionEnded:
			a.result.Ended = true
		case api.CodeCommandFailed, api.CodeBadCommand, api.CodeBadSize:
			// A refused request changes nothing; the screen shows why.
			return a, a.screen.Update(msg)
		default:
			a.result.Err = msg
		}
		return a, tea.Quit
	case connClosedMsg:
		if !a.result.Ended && !a.result.Detached && a.result.Err == nil {
			a.result.Err = ErrConnectionLost
		}
		return a, tea.Quit
	case home.OpenMsg:
		a.send(&api.Attach{Session: msg.Session, Create: msg.Create, Dir: a.dir})
		a.result.Detached = false
		a.show(a.newSession())
		return a, nil
	case home.QuitMsg:
		return a, tea.Quit
	case session.DetachMsg:
		a.result.Detached = true
		a.leave("")
		return a, nil
	case session.SwitchMsg:
		a.leave(a.result.Session)
		return a, nil
	}
	return a, a.screen.Update(msg)
}

// View shows the screen and names the session in the terminal's title, so
// the tab says which session it is. Bubble Tea clears the title on exit.
func (a *app) View() tea.View {
	v := a.screen.View()
	v.AltScreen = true
	v.WindowTitle = "snow"
	if a.result.Session != "" {
		v.WindowTitle = "snow: " + a.result.Session
	}
	return v
}
