package client

import (
	"errors"
	"image/color"
	"time"

	"github.com/emmyfong/snow/internal/client/command"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/home"
	"github.com/emmyfong/snow/internal/client/keys"
	"github.com/emmyfong/snow/internal/client/newsession"
	"github.com/emmyfong/snow/internal/client/session"
	"github.com/emmyfong/snow/internal/client/settings"
	"github.com/emmyfong/snow/internal/client/settings/panegroup"
	"github.com/emmyfong/snow/internal/client/settings/themegroup"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/config"
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
	screen         Screen
	back           Screen // the screen settings temporarily covers
	send           func(api.Message)
	result         Result
	dir            string           // the folder a session opened from the home page starts in
	version        string           // Snow's version, for the home page
	theme          theme.Theme      // the theme currently shown by the client
	configTheme    theme.Theme      // the last theme loaded from or saved to config
	sessOpts       []session.Option // what a session screen gets besides the theme, such as keys
	keys           *keys.Table      // the key table, so the home page names the prefix; nil means the defaults
	saveTheme      func(name string) error
	configPath     string    // the config file to watch for a new theme; "" watches nothing
	configMod      time.Time // when the config file last changed, as last seen
	settingsLoadID uint64    // rejects config snapshots from an earlier settings screen
	profile        *tea.ColorProfileMsg
	sessions       *api.SessionList // the last list, for a screen opened later
	agents         *api.AgentList   // the last agent list, for a screen opened later
	opening        bool             // a session is being opened; its Layout has not come
	openName       string           // the session being opened; "" for a new numbered one
	leftName       string           // the session shown when the opening began
	runtimeProbeID string           // home-page Claude readiness request still in flight
	focused        bool             // the terminal has focus
	focusReport    bool             // the terminal reports focus changes; see api.ClientFocus
	cols, rows     int

	terminalBackground bool
	originalForeground color.Color
	originalBackground color.Color
	paintedDefaults    bool
}

// newHomeApp returns an app that starts on the home page. Opening a session
// there attaches and switches to the session screen. A terminal starts in the
// foreground, and many send no event when Bubble Tea starts reporting focus,
// so the app starts focused. It asks the terminal whether it reports focus
// before it sets View.ReportFocus; an unsupported terminal reports that it is
// not focused, so the server never mistakes it for a watching user.
func newHomeApp(send func(api.Message), version, dir string, th theme.Theme, opts ...session.Option) *app {
	h := home.New(version, th)
	h.SetFolder(dir)
	return &app{screen: h, send: send, dir: dir, version: version, theme: th, configTheme: th, sessOpts: opts, focused: true}
}

// setKeys gives the home page, now and when it comes back, the key table.
func (a *app) setKeys(t *keys.Table) {
	a.keys = t
	for _, screen := range []Screen{a.screen, a.back} {
		if h, ok := screen.(*home.Model); ok {
			h.SetKeys(t)
		}
	}
}

// newSession returns a session screen with the app's theme and options.
func (a *app) newSession() *session.Model {
	return session.New(a.send, append([]session.Option{session.WithTheme(a.theme)}, a.sessOpts...)...)
}

func (a *app) screenForEvents() Screen {
	if a.back != nil {
		return a.back
	}
	return a.screen
}

func (a *app) updateScreen(msg tea.Msg) tea.Cmd {
	target, currentScreen := a.screen, true
	if a.back != nil {
		switch msg.(type) {
		case tea.KeyPressMsg, tea.MouseMsg, tea.WindowSizeMsg:
		default:
			target, currentScreen = a.back, false
		}
	}
	cmd := target.Update(msg)
	if currentScreen {
		if current, ok := target.(interface{ CurrentTheme() theme.Theme }); ok {
			a.theme = current.CurrentTheme()
			a.setScreenBackground(target)
		}
	}
	return cmd
}

// themeSaveFailedMsg reports that a theme was applied but not saved.
type themeSaveFailedMsg struct{ err error }

type settingsConfigMsg struct {
	id  uint64
	cfg config.Config
	err error
}

func loadSettingsConfig(path string, id uint64) tea.Cmd {
	return func() tea.Msg {
		if path == "" {
			return settingsConfigMsg{id: id, cfg: config.Defaults()}
		}
		cfg, err := config.Load(path)
		if err != nil {
			cfg = config.Defaults()
		}
		return settingsConfigMsg{id: id, cfg: cfg, err: err}
	}
}

// useTheme makes the theme called name the app's, for screens opened from
// now on, and saves it in the background: writing a file must not hold up
// the screen.
func (a *app) useTheme(name string) tea.Cmd {
	th, err := theme.Named(name)
	if err != nil {
		return nil
	}
	a.theme = th
	a.configTheme = th
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
	a.setScreenBackground(s)
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
	a.back = nil
	a.theme = a.configTheme
	h := home.New(a.version, a.theme)
	h.SetFolder(a.dir)
	if a.keys != nil {
		h.SetKeys(a.keys)
	}
	h.SetNotice(notice)
	a.show(h)
	if a.sessions != nil {
		h.Update(a.sessions)
	}
}

// attach asks the server for session, creating it when create is set, and
// then reports where the terminal stands. The server skips a notification
// for a pane a focused client shows, so it must not wait for a later focus
// event: many terminals send none until the focus changes.
func (a *app) attach(session string, create bool, dir string, pane int) {
	if dir == "" {
		dir = a.dir
	}
	a.send(&api.Attach{Session: session, Create: create, Dir: dir, Pane: pane})
	a.send(&api.ClientFocus{Focused: a.focused})
}

// open attaches to session and shows a new session screen for it. Pane,
// when set, is a pane of that session to focus, as when the user picks an
// agent in the needs-you inbox.
func (a *app) open(session string, create bool, dir, layout string, pane int, runtimeConnection string) {
	a.back = nil
	s := a.newSession()
	if create && layout != "" {
		s.SetPendingLayout(layout)
	}
	if create && runtimeConnection != "" {
		s.SetPendingRuntime(runtimeConnection)
	}
	a.attach(session, create, dir, pane)
	a.opening, a.openName, a.leftName = true, session, a.result.Session
	a.result.Detached = false
	a.show(s)
	if a.sessions != nil {
		s.Update(a.sessions)
	}
	// The server sends the agent list only when it changes, so a detach
	// keeps it current.
	if a.agents != nil {
		s.Update(a.agents)
	}
}

// attachStart attaches to the session Run started for and shows its screen.
// It reports focus too, so a client started in the foreground never gets a
// notification for the pane it shows.
func (a *app) attachStart(session string) {
	a.back = nil
	a.attach(session, true, "", 0)
	a.screen = a.newSession()
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
	// A server predating folder-based runtime lists refuses the home-page
	// readiness probe with this error while Attach may still be succeeding.
	if e.Code == api.CodeBadCommand && e.Message == api.ErrInvalidRuntime.Error() {
		return false
	}
	switch e.Code {
	case api.CodeNoSession, api.CodeAttachFailed, api.CodeBadCommand:
		return true
	}
	return false
}

func (a *app) Init() tea.Cmd {
	// Ask whether the terminal reports focus events. Some do not, and some
	// multiplexers need them turned on. Until an answer says otherwise, the
	// app keeps View.ReportFocus false.
	cmds := []tea.Cmd{tea.Raw(ansi.RequestModeFocusEvent), tea.RequestForegroundColor, tea.RequestBackgroundColor}
	if a.configPath != "" {
		cmds = append(cmds, watchTheme(a.configPath, a.configMod))
	}
	return tea.Batch(cmds...)
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
		a.trackDefaultColors()
		if a.back != nil {
			a.screen.Update(msg)
			return a, a.back.Update(msg)
		}
	case tea.ForegroundColorMsg:
		if a.originalForeground == nil {
			a.originalForeground = msg.Color
		}
		a.trackDefaultColors()
		return a, nil
	case tea.BackgroundColorMsg:
		if a.originalBackground == nil {
			a.originalBackground = msg.Color
		}
		a.trackDefaultColors()
		return a, nil
	case tea.ModeReportMsg:
		if msg.Mode != ansi.ModeFocusEvent {
			return a, nil
		}
		a.focusReport = !msg.Value.IsNotRecognized()
		if !a.focusReport {
			// Without focus reports a smart terminal cannot be told from a
			// background one, so the server must never skip a notification.
			a.focused = false
			a.send(&api.ClientFocus{Focused: false})
		}
		return a, nil
	case tea.FocusMsg:
		a.focused = true
		a.send(&api.ClientFocus{Focused: true})
		return a, nil
	case tea.BlurMsg:
		a.focused = false
		a.send(&api.ClientFocus{Focused: false})
		return a, nil
	case *api.SessionList:
		a.sessions = msg
	case *api.AgentList:
		a.agents = msg
	case *api.RuntimeConnections:
		if msg.Pane == 0 && msg.RequestID == a.runtimeProbeID {
			a.runtimeProbeID = ""
		}
		return a, a.updateScreen(msg)
	case *api.Layout:
		if a.staleLayout(msg) {
			return a, nil
		}
		a.result.Session = msg.Session
		a.opening = false
	case *api.Error:
		if a.runtimeProbeID != "" && msg.Code == api.CodeBadCommand && msg.Message == api.ErrInvalidRuntime.Error() {
			a.runtimeProbeID = ""
			return a, nil
		}
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
			if _, on := a.screenForEvents().(*session.Model); !on || a.opening {
				return a, nil
			}
			// The server keeps the connection, detached; the session and
			// the list naming it are gone.
			a.result, a.sessions = Result{}, nil
			a.showHome(msg.Message)
			return a, nil
		case api.CodeBadHandshake, api.CodeNoSession, api.CodeAttachFailed:
			// No session is left to show: end the client with the reason.
			a.result.Err = msg
			return a, tea.Quit
		default:
			// A refused request changes nothing, and a notice is only
			// information; the screen shows either. A refused login goes
			// to the login picker, which keeps the key, or to the status
			// bar when no picker waits. A code this client does not know
			// stays here too: ending on it would turn each new code into
			// a crash until someone listed it (#238).
			return a, a.updateScreen(msg)
		}
	case connClosedMsg:
		if !a.result.Detached && a.result.Err == nil {
			a.result.Err = ErrConnectionLost
		}
		return a, tea.Quit
	case home.OpenMsg:
		connection := ""
		if msg.Runtime == newsession.StartClaudeCode {
			connection = msg.Connection
		}
		a.open(msg.Session, msg.Create, msg.Dir, msg.Layout, 0, connection)
		if msg.Create && msg.Runtime == newsession.StartShell {
			if s, ok := a.screen.(*session.Model); ok {
				s.SetPendingShell()
			}
		}
		return a, nil
	case home.SendMsg:
		if request, ok := msg.Msg.(*api.RuntimeRequest); ok && request.Pane == 0 && request.Action == api.RuntimeList {
			a.runtimeProbeID = request.RequestID
		}
		a.send(msg.Msg)
		return a, nil
	case command.ThemeMsg:
		// The open screen draws with it at once; screens opened later
		// get it from useTheme.
		if th, err := theme.Named(msg.Name); err == nil {
			a.applyTheme(th)
		}
		return a, a.useTheme(msg.Name)
	case command.PreviewThemeMsg:
		current, ok := a.screen.(interface{ CurrentTheme() theme.Theme })
		if !ok || current.CurrentTheme().Name != msg.Theme.Name {
			return a, nil
		}
		a.previewTheme(msg.Theme)
		return a, nil
	case command.SettingsMsg:
		if a.back == nil {
			a.settingsLoadID++
			s := settings.New([]settings.Group{themegroup.New(a.theme)})
			s.SetTheme(a.theme)
			a.back = a.screen
			a.show(s)
			return a, loadSettingsConfig(a.configPath, a.settingsLoadID)
		}
		return a, nil
	case settingsConfigMsg:
		if s, ok := a.screen.(*settings.Model); ok && a.back != nil && msg.id == a.settingsLoadID {
			s.AddGroups([]settings.Group{
				panegroup.NewProfiles(msg.cfg),
				panegroup.NewScrollback(msg.cfg),
			})
			if msg.err != nil {
				s.SetNotice("Using defaults. Config could not be read: " + msg.err.Error())
			}
		}
		return a, nil
	case settings.CloseMsg:
		if a.back != nil {
			a.settingsLoadID++
			s := a.back
			a.back = nil
			a.show(s)
		}
		return a, nil
	case configCheckedMsg:
		a.configMod = msg.mod
		if msg.background != "" {
			a.terminalBackground = msg.background == config.BackgroundTerminal
			a.trackDefaultColors()
		}
		if msg.th != nil && msg.th.Name != a.configTheme.Name {
			if s, ok := a.screen.(*settings.Model); ok {
				s.SetCommittedTheme(*msg.th)
			}
			a.applyTheme(*msg.th)
		}
		if a.configPath == "" {
			return a, nil
		}
		return a, watchTheme(a.configPath, a.configMod)
	case themeSaveFailedMsg:
		text := "The theme is in use but was not saved: " + msg.err.Error()
		if s, ok := a.screen.(*settings.Model); ok {
			s.SetNotice(text)
			return a, nil
		}
		switch s := a.screenForEvents().(type) {
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
		a.open(msg.Session, msg.Create, "", "", msg.Pane, "")
		return a, nil
	}
	return a, a.updateScreen(msg)
}

// View shows the screen and names the session in the terminal's title, so
// the tab says which session it is. Bubble Tea clears the title on exit.
func (a *app) View() tea.View {
	v := a.screen.View()
	v.BackgroundColor, v.ForegroundColor = a.defaultColors()
	v.AltScreen = true
	// Focus and blur events tell the server whether the user looks at an
	// agent's pane, so it can skip a notification for it. Ask for them only
	// once the terminal says it reports them.
	v.ReportFocus = a.focusReport
	v.WindowTitle = "snow"
	if a.result.Session != "" {
		v.WindowTitle = "snow: " + a.result.Session
	}
	return v
}
