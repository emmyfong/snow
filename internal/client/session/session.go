// Package session is the attached view: the panes of the shown window, their
// borders, and the status bar.
package session

import (
	"github.com/charmbracelet/colorprofile"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/authpick"
	"github.com/emmyfong/snow/internal/client/clipboard"
	"github.com/emmyfong/snow/internal/client/copymode"
	"github.com/emmyfong/snow/internal/client/keys"
	"github.com/emmyfong/snow/internal/client/menu"
	"github.com/emmyfong/snow/internal/client/modelpick"
	"github.com/emmyfong/snow/internal/client/mouse"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/internal/client/resumepick"
	"github.com/emmyfong/snow/internal/client/sessionlist"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// DetachMsg asks the client to detach from the session and show the home
// page.
type DetachMsg struct{}

// Model is the session screen. Send queues a message for the server; it must
// keep order.
type Model struct {
	send       func(api.Message)
	theme      theme.Theme           // base adapted to the terminal's colors
	base       theme.Theme           // the theme from the config
	profile    *colorprofile.Profile // the terminal's colors, once known
	keys       *keys.Table
	now        func() time.Time
	layout     *api.Layout
	panes      map[int]*paneState
	profiles   api.ProfileList
	sessions   []api.SessionInfo // the server's last session list
	agents     []api.AgentInfo   // the server's last agent list
	cols, rows int
	prefixed   bool    // the prefix key was pressed; the next key is a command
	prefixKey  tea.Key // the key that started the prefix, for send-prefix
	prefixSeq  int     // counts prefix presses, to match a hint timer to its press
	hint       bool    // the which-key popup is showing
	hintDelay  time.Duration
	early      []api.Input // input typed before the first Layout named a pane
	message    string      // why the last request was refused
	messageAt  time.Time
	notice     string // a note on what the user did, such as a copy; not a refusal
	noticeAt   time.Time
	ticking    bool // the clock tick is running
	picker     *picker.Model
	switcher   *sessionlist.Model        // the session list, when open
	prompt     *prompting                // the command popup, when open
	confirm    *confirmation             // a question waiting for y or n
	rename     *renaming                 // the window name being typed
	menu       *menu.Model               // the right-click menu
	drag       *mouse.Drag               // a border held by the pointer
	grab       *grab                     // the pane a held button belongs to
	pending    api.Command               // the command the open picker completes
	copy       *copymode.Model           // copy mode over a pane, when open
	setClip    func(text string) tea.Cmd // puts text on the system clipboard
	dragFrom   *paneDrag                 // a left press in a pane, until it moves or lets go
	own        map[int]ownPane           // pane ID → pane the client draws, such as a chat
	modelPick  *modelpick.Model          // the model picker, when open
	modelList  api.ModelList             // the server's last model list
	modelPane  int                       // the pane modelPick chooses a model for
	firstRun   map[int]bool              // panes the first-run flow ran for
	over       *overlay                  // the mode picker, resume picker, or help, when open
	resume     *resumepick.Model         // the resume picker, when over is it
	authPick   *authpick.Model           // the login picker, when open
	authPane   int                       // the pane authPick serves
	authList   api.AuthList              // the server's last provider key list
	authWait   map[int]bool              // panes waiting for the first-run key list
	authAfter  map[int]bool              // panes to open the model picker for after a login
	loginPane  int                       // the pane whose login check runs, or 0
	logoutPane int                       // the pane whose /logout waits for a key list, or 0
	// ownTicking is set while a tick for animating panes is pending.
	ownTicking bool
	hideUsage  bool // chats leave out the context gauge and the cost
}

// maxEarly bounds input held before the first Layout. The Layout follows
// Attach at once, so only a fast typist or a paste can fill it.
const maxEarly = 256

// messageFor is how long a refused request's message stays in the status
// bar.
const messageFor = 3 * time.Second

// clockEvery is how often the status bar's clock is redrawn.
const clockEvery = 15 * time.Second

type clearMessageMsg struct{ at time.Time }

type clearNoticeMsg struct{ at time.Time }

type clockMsg struct{}

// New returns a session screen that sends input through send.
func New(send func(api.Message), opts ...Option) *Model {
	m := &Model{send: send, theme: theme.Default(), base: theme.Default(), keys: keys.Default(), hintDelay: hintDelay, now: time.Now,
		panes: map[int]*paneState{}, own: map[int]ownPane{}, firstRun: map[int]bool{}, authWait: map[int]bool{},
		authAfter: map[int]bool{}, setClip: clipboard.Set}
	for _, o := range opts {
		o(m)
	}
	return m
}

// SetSize sets the screen size in cells.
func (m *Model) SetSize(cols, rows int) { m.cols, m.rows = cols, rows }

// Update applies server messages and handles keys.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case *api.Layout:
		m.applyLayout(msg)
		if !m.ticking {
			m.ticking = true
			return tick()
		}
	case *api.PaneUpdate:
		m.applyUpdate(msg)
	case *api.ChatSnapshot, *api.ChatEvent, ownTickMsg:
		return m.ownMsg(msg)
	case tea.ColorProfileMsg:
		// The terminal's color depth is known only now; text that would
		// vanish when colors are reduced switches to black or white.
		m.profile = &msg.Profile
		m.theme = m.base.For(msg.Profile)
		for _, p := range m.own {
			p.SetTheme(m.theme)
		}
	case *api.ProfileList:
		m.profiles = *msg
	case *api.ModelList:
		m.setModelList(msg)
	case *api.ConversationList:
		m.setConversationList(msg)
		if p := m.own[msg.Pane]; p != nil {
			p.Feed(msg)
		}
	case *api.AuthList:
		return m.setAuthList(msg)
	case *api.Login:
		return m.loginDone(msg)
	case *api.Logout:
		return m.logoutDone(msg)
	case *api.AgentList:
		m.setAgents(msg)
	case *api.SessionList:
		m.sessions = msg.Sessions
		if m.switcher != nil {
			m.switcher.SetSessions(msg.Sessions)
		}
	case *api.Scrollback:
		return m.copyReceive(msg)
	case *api.SearchResult:
		return m.copyReceive(msg)
	case clipboard.ErrMsg:
		return m.ShowError(msg.Error())
	case *api.Error:
		if msg.Code == api.CodeNotice {
			return m.showNotice(msg.Message)
		}
		if cmd, ok := m.authError(msg.Message); ok {
			return cmd
		}
		return m.ShowError(msg.Message)
	case clearMessageMsg:
		if msg.at.Equal(m.messageAt) {
			m.message = ""
		}
	case clearNoticeMsg:
		if msg.at.Equal(m.noticeAt) {
			m.notice = ""
		}
	case hintMsg:
		m.showHint(msg)
	case clockMsg:
		return tick()
	case tea.KeyPressMsg:
		return m.key(msg.Key())
	case tea.MouseMsg:
		return m.mouseMsg(msg)
	case tea.PasteMsg:
		switch c := m.focusedOwn(); {
		case m.modal():
		case c != nil:
			c.Paste(msg.Content)
		default:
			m.input(api.Input{Paste: msg.Content})
		}
	}
	return nil
}

// ShowError shows text in the status bar for a moment: why a request was
// refused, or why something the user asked for cannot happen.
func (m *Model) ShowError(text string) tea.Cmd {
	m.message, m.messageAt = text, m.now()
	at := m.messageAt
	return tea.Tick(messageFor, func(time.Time) tea.Msg { return clearMessageMsg{at} })
}

// showNotice shows text in the status bar for a moment: something that
// worked, such as a saved key.
func (m *Model) showNotice(text string) tea.Cmd {
	m.notice, m.noticeAt = text, m.now()
	at := m.noticeAt
	return tea.Tick(messageFor, func(time.Time) tea.Msg { return clearNoticeMsg{at} })
}

func tick() tea.Cmd {
	return tea.Tick(clockEvery, func(time.Time) tea.Msg { return clockMsg{} })
}

// shown returns the window on screen, or nil before the first Layout.
func (m *Model) shown() *api.WindowInfo {
	if m.layout == nil {
		return nil
	}
	for i := range m.layout.Windows {
		if m.layout.Windows[i].Index == m.layout.Active {
			return &m.layout.Windows[i]
		}
	}
	return nil
}

// focused returns the focused pane of the shown window, or nil. A layout
// that marks no pane focused, as from a server before #53, focuses the
// first pane.
func (m *Model) focused() *api.PaneInfo {
	w := m.shown()
	if w == nil || len(w.Panes) == 0 {
		return nil
	}
	for i := range w.Panes {
		if w.Panes[i].Focused {
			return &w.Panes[i]
		}
	}
	return &w.Panes[0]
}

// input sends in to the focused pane. Before the first Layout there is no
// pane yet, so input waits; dropping it would lose what the user typed right
// after starting snow.
func (m *Model) input(in api.Input) {
	p := m.focused()
	if p == nil {
		if len(m.early) < maxEarly {
			m.early = append(m.early, in)
		}
		return
	}
	in.Pane = p.ID
	m.send(&in)
}

func (m *Model) applyLayout(l *api.Layout) {
	m.layout = l
	defer m.syncCopy()
	defer m.syncDrag()
	defer m.syncPanes()
	present := map[int]bool{}
	for _, w := range l.Windows {
		for _, p := range w.Panes {
			present[p.ID] = true
			st, ok := m.panes[p.ID]
			if !ok {
				st = &paneState{}
				m.panes[p.ID] = st
			}
			st.lines = resize(st.lines, p.Rect.H)
		}
	}
	for id := range m.panes {
		if !present[id] {
			delete(m.panes, id)
		}
	}
	if m.focused() != nil {
		early := m.early
		m.early = nil
		for _, in := range early {
			m.input(in)
		}
	}
}

// modal reports whether a popup or a question holds the keyboard.
func (m *Model) modal() bool {
	return m.picker != nil || m.switcher != nil || m.modelPick != nil || m.authPick != nil || m.over != nil || m.prompt != nil || m.confirm != nil || m.rename != nil || m.menu != nil || m.copy != nil
}
