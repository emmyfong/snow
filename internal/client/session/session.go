// Package session is the attached view: the panes of the shown window, their
// borders, and the status bar.
package session

import (
	"github.com/charmbracelet/colorprofile"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/clipboard"
	"github.com/emmyfong/snow/internal/client/copymode"
	"github.com/emmyfong/snow/internal/client/keys"
	"github.com/emmyfong/snow/internal/client/menu"
	"github.com/emmyfong/snow/internal/client/mouse"
	"github.com/emmyfong/snow/internal/client/picker"
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
}

// paneState is what the screen knows of one pane: its rows and its cursor.
type paneState struct {
	lines  []string
	cursor api.Cursor
	mouse  bool // the program asked for mouse events
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
		panes: map[int]*paneState{}, setClip: clipboard.Set}
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
	case tea.ColorProfileMsg:
		// The terminal's color depth is known only now; text that would
		// vanish when colors are reduced switches to black or white.
		m.profile = &msg.Profile
		m.theme = m.base.For(msg.Profile)
	case *api.ProfileList:
		m.profiles = *msg
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
		return m.refuse(msg.Error())
	case *api.Error:
		return m.refuse(msg.Message)
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
		if !m.modal() {
			m.input(api.Input{Paste: msg.Content})
		}
	}
	return nil
}

// ShowError shows text in the status bar for a moment, as a refused
// request's reason is shown.
func (m *Model) ShowError(text string) tea.Cmd { return m.refuse(text) }

// refuse shows why a request failed in the status bar for a moment.
func (m *Model) refuse(text string) tea.Cmd {
	m.message, m.messageAt = text, m.now()
	at := m.messageAt
	return tea.Tick(messageFor, func(time.Time) tea.Msg { return clearMessageMsg{at} })
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

func (m *Model) applyUpdate(u *api.PaneUpdate) {
	st, ok := m.panes[u.Pane]
	if !ok {
		return
	}
	for _, l := range u.Lines {
		if l.Row < 0 {
			continue
		}
		if l.Row >= len(st.lines) {
			st.lines = resize(st.lines, l.Row+1)
		}
		st.lines[l.Row] = l.Text
	}
	if u.Cursor != nil {
		st.cursor = *u.Cursor
	}
	st.mouse = u.Mouse
}

func resize(lines []string, rows int) []string {
	if rows < 0 {
		rows = 0
	}
	if rows <= len(lines) {
		return lines[:rows]
	}
	return append(lines, make([]string, rows-len(lines))...)
}

// modal reports whether a popup or a question holds the keyboard.
func (m *Model) modal() bool {
	return m.picker != nil || m.switcher != nil || m.prompt != nil || m.confirm != nil || m.rename != nil || m.menu != nil || m.copy != nil
}
