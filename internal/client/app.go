package client

import (
	"errors"
	"time"

	"github.com/emmyfong/snow/internal/client/command"

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
	sessOpts   []session.Option // what a session screen gets besides the theme, such as keys
	saveTheme  func(name string) error
	configPath string    // the config file to watch for a new theme; "" watches nothing
	configMod  time.Time // when the config file last changed, as last seen
	profile    *tea.ColorProfileMsg
	sessions   *api.SessionList // the last list, for a screen opened later
	opening    bool             // a session is being opened; its Layout has not come
	openName   string           // the session being opened; "" for a new numbered one
	leftName   string           // the session shown when the opening began
	cols, rows int
}

// newHomeApp returns an app that starts on the home page. Opening a session
// there attaches and switches to the session screen.
func newHomeApp(send func(api.Message), version, dir string, th theme.Theme, opts ...session.Option) *app {
	return &app{screen: home.New(version, th), send: send, dir: dir, version: version, theme: th, sessOpts: opts}
}

// newSession returns a session screen with the app's theme and options.
func (a *app) newSession() *session.Model {
	return session.New(a.send, append([]session.Option{session.WithTheme(a.theme)}, a.sessOpts...)...)
}

// themeSaveFailedMsg reports that a theme was applied but not saved.
type themeSaveFailedMsg struct{ err error }

// useTheme makes the theme called name the app's, for screens opened from
// now on, and saves it in the background: writing a file must not hold up
// the screen.
func (a *app) useTheme(name string) tea.Cmd {
	th, err := theme.Named(name)
	if err != nil {
		return nil
	}
	a.theme = th
	save := a.saveTheme
	if save == nil {
		return nil
	}
	return func() tea.Msg {
		if err := save(th.Name); err != nil {
			return themeSaveFailedMsg{err}
		}
		return nil
	}
}

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
// page.
func (a *app) leave() {
	a.send(&api.Detach{})
	// The list from before attaching is old; the server sends a new one
	// for the Detach.
	a.sessions = nil
	a.showHome("")
}

// showHome shows a new home page with the last session list and notice, if
// any.
func (a *app) showHome(notice string) {
	h := home.New(a.version, a.theme)
	h.SetNotice(notice)
	a.show(h)
	if a.sessions != nil {
		h.Update(a.sessions)
	}
}

// open attaches to session, creating it when create is set, and shows a
// new session screen for it.
func (a *app) open(session string, create bool) {
	a.send(&api.Attach{Session: session, Create: create, Dir: a.dir})
	a.opening, a.openName, a.leftName = true, session, a.result.Session
	a.result.Detached = false
	s := a.newSession()
	a.show(s)
	if a.sessions != nil {
		s.Update(a.sessions)
	}
}

// staleLayout reports whether l belongs to another session than the one
// being opened: a Layout of the session left, sent before the server saw
// the Detach. Taking it would end the opening early, so a refused attach
// would no longer be recognized.
func (a *app) staleLayout(l *api.Layout) bool {
	if !a.opening {
		return false
	}
	if a.openName != "" {
		return l.Session != a.openName
	}
	return a.leftName != "" && l.Session == a.leftName
}

// refusedOpen reports whether e refuses the attach the home page asked for.
// The server answers an attach with a Layout or with one of these.
func (a *app) refusedOpen(e *api.Error) bool {
	if !a.opening {
		return false
	}
	switch e.Code {
	case api.CodeNoSession, api.CodeAttachFailed, api.CodeBadCommand:
		return true
	}
	return false
}

func (a *app) Init() tea.Cmd {
	if a.configPath == "" {
		return nil
	}
	return watchTheme(a.configPath, a.configMod)
}

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
	case *api.SessionList:
		a.sessions = msg
	case *api.Layout:
		if a.staleLayout(msg) {
			return a, nil
		}
		a.result.Session = msg.Session
		a.opening = false
	case *api.Error:
		if a.refusedOpen(msg) {
			// The session ended, or the name was refused, between the
			// list and the click: go back to choosing.
			a.opening = false
			a.showHome(msg.Message)
			return a, nil
		}
		switch msg.Code {
		case api.CodeSessionEnded:
			// An end sent before the user left the session arrives after:
			// the home page or the session being opened stays.
			if _, on := a.screen.(*session.Model); !on || a.opening {
				return a, nil
			}
			// The server keeps the connection, detached; the session and
			// the list naming it are gone.
			a.result, a.sessions = Result{}, nil
			a.showHome(msg.Message)
			return a, nil
		case api.CodeCommandFailed, api.CodeBadCommand, api.CodeBadSize, api.CodeAuthFailed:
			// A refused request changes nothing; the screen shows why. A
			// refused login goes to the login picker, which keeps the key,
			// or to the status bar when no picker waits.
			return a, a.screen.Update(msg)
		default:
			a.result.Err = msg
		}
		return a, tea.Quit
	case connClosedMsg:
		if !a.result.Detached && a.result.Err == nil {
			a.result.Err = ErrConnectionLost
		}
		return a, tea.Quit
	case home.OpenMsg:
		a.open(msg.Session, msg.Create)
		return a, nil
	case home.SendMsg:
		a.send(msg.Msg)
		return a, nil
	case command.ThemeMsg:
		// The open screen draws with it at once; screens opened later
		// get it from useTheme.
		if th, err := theme.Named(msg.Name); err == nil {
			a.applyTheme(th)
		}
		return a, a.useTheme(msg.Name)
	case configCheckedMsg:
		a.configMod = msg.mod
		if msg.th != nil && msg.th.Name != a.theme.Name {
			a.applyTheme(*msg.th)
		}
		if a.configPath == "" {
			return a, nil
		}
		return a, watchTheme(a.configPath, a.configMod)
	case themeSaveFailedMsg:
		text := "The theme is in use but was not saved: " + msg.err.Error()
		switch s := a.screen.(type) {
		case *home.Model:
			s.SetNotice(text)
		case *session.Model:
			return a, s.ShowError(text)
		}
		return a, nil
	case home.QuitMsg:
		return a, tea.Quit
	case session.DetachMsg:
		a.result.Detached = true
		a.leave()
		return a, nil
	case session.SwitchMsg:
		// Detach first: if the attach is refused, the client is left
		// attached to nothing, which matches the home page it then shows.
		a.send(&api.Detach{})
		a.open(msg.Session, msg.Create)
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
