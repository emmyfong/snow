// Package home is the home page: Snow's wordmark, an input box to open or
// create a session, the running sessions, and key hints. It is the screen
// plain `snow` opens, and it can be replaced without touching other screens.
package home

import (
	"github.com/charmbracelet/colorprofile"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/command"
	"github.com/emmyfong/snow/internal/client/confirm"
	"github.com/emmyfong/snow/internal/client/keys"
	"github.com/emmyfong/snow/internal/client/sessionlist"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// OpenMsg asks the client to attach to Session, creating it when Create is
// set. An empty Session with Create starts a numbered session.
type OpenMsg struct {
	Session string
	Create  bool
}

// QuitMsg asks the client to leave Snow.
type QuitMsg struct{}

// Model is the home page.
type Model struct {
	version    string
	theme      theme.Theme // base adapted to the terminal's colors
	base       theme.Theme // the theme from the config
	now        func() time.Time
	sessions   []api.SessionInfo // most recently used first
	input      string
	selected   int      // index into the shown sessions; -1 means none
	target     string   // the session highlighted when a slash command began
	notes      []string // problems with saved sessions, from the server
	cmdSel     int      // index into the matching commands
	notice     string   // why the last command or request was refused
	help       bool     // the help panel is open
	naming     *naming  // the new name /rename asks for, when open
	confirm    *confirm.Model
	killing    string // the session the open popup would end
	picking    bool   // the theme list is open
	themeSel   int    // index into theme.Builtin
	before     theme.Theme
	profile    *colorprofile.Profile // the terminal's colors, once known
	cols, rows int
}

// New returns the home page for Snow version, drawn with th.
func New(version string, th theme.Theme) *Model {
	return &Model{version: version, theme: th, base: th, now: time.Now, selected: -1}
}

// SetNotice shows text under the input, such as why a session could not be
// opened.
func (m *Model) SetNotice(text string) { m.notice = text }

// SetSize sets the screen size in cells.
func (m *Model) SetSize(cols, rows int) { m.cols, m.rows = cols, rows }

// Update handles the session list, keys, and clicks.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.ColorProfileMsg:
		m.profile = &msg.Profile
		m.theme = m.base.For(msg.Profile)
	case *api.SessionList:
		m.notes = msg.Notes
		chosen := ""
		if shown := m.shown(); m.selected >= 0 && m.selected < len(shown) {
			chosen = shown[m.selected].Name
		}
		m.sessions = sessionlist.Sort(msg.Sessions)
		// The new list may be in another order: keep the highlight on the
		// same session, or drop it when that session is gone.
		m.selected = slices.IndexFunc(m.shown(), func(s api.SessionInfo) bool { return s.Name == chosen })
	case *api.Error:
		// A refused rename or kill; the server says why.
		m.notice = msg.Message
	case tea.KeyPressMsg:
		return m.key(msg.Key())
	case tea.MouseClickMsg:
		return m.click(msg)
	}
	return nil
}

// click handles a left click: a popup button, a command row (completed,
// or run when it takes no argument), or a session row (opened, or picked
// and the command run, under a command that acts on a session).
func (m *Model) click(msg tea.MouseClickMsg) tea.Cmd {
	if msg.Button != tea.MouseLeft {
		return nil
	}
	if m.confirm != nil {
		return m.confirmResult(m.confirm.Click(m.theme, m.cols, m.rows, msg.X, msg.Y))
	}
	if m.help {
		m.help = false
		return nil
	}
	if m.naming != nil {
		return nil
	}
	p := m.layout()
	if i, ok := p.sessions.rowAt(msg.Y); ok && m.pickingSession() {
		m.target = m.sessions[i].Name
		return m.runCommand()
	}
	i, ok := p.list.rowAt(msg.Y)
	if !ok {
		return nil
	}
	if m.picking {
		m.pickTheme(i)
		return m.keepTheme()
	}
	if m.commanding() {
		c := m.matches()[i]
		m.complete(c)
		if c.Args == "" {
			return m.runCommand()
		}
		return nil
	}
	return openCmd(OpenMsg{Session: m.shown()[i].Name})
}

func (m *Model) key(k tea.Key) tea.Cmd {
	if m.confirm != nil {
		return m.confirmResult(m.confirm.Key(k))
	}
	if m.picking {
		return m.pickerKey(k)
	}
	if m.help {
		// Any key closes the help; it holds nothing to act on.
		m.help = false
		return nil
	}
	m.notice = ""
	if m.naming != nil {
		return m.namingKey(k)
	}
	if m.commanding() {
		if cmd, ok := m.commandKey(k); ok {
			return cmd
		}
	}
	switch {
	case k.Code == 'c' && k.Mod == tea.ModCtrl:
		return func() tea.Msg { return QuitMsg{} }
	case k.Code == tea.KeyEscape:
		if m.input == "" {
			return func() tea.Msg { return QuitMsg{} }
		}
		m.input, m.selected, m.target, m.cmdSel = "", -1, "", 0
	case k.Code == tea.KeyEnter:
		return m.open(m.choice())
	case k.Code == tea.KeyUp:
		if m.selected > 0 {
			m.selected--
		}
	case k.Code == tea.KeyDown:
		if m.selected < len(m.shown())-1 {
			m.selected++
		}
	default:
		if s, ok := keys.Edit(m.input, k); ok {
			m.setInput(s)
		}
	}
	return nil
}

// setInput changes the input to s. A "/" that starts the input begins a
// command, which acts on the session highlighted now; editing clears the
// highlight.
func (m *Model) setInput(s string) {
	if m.input == "" && strings.HasPrefix(s, "/") {
		m.target = ""
		if shown := m.shown(); m.selected >= 0 && m.selected < len(shown) {
			m.target = shown[m.selected].Name
		}
	}
	m.input, m.selected, m.cmdSel = s, -1, 0
}

// choice is what Enter opens: the selected session, else the typed name
// (attached if it exists, created if not), else a new numbered session.
func (m *Model) choice() OpenMsg {
	if shown := m.shown(); m.selected >= 0 && m.selected < len(shown) {
		return OpenMsg{Session: shown[m.selected].Name}
	}
	name := strings.TrimSpace(m.input)
	if name == "" {
		return OpenMsg{Create: true}
	}
	for _, s := range m.sessions {
		if s.Name == name {
			return OpenMsg{Session: name}
		}
	}
	return OpenMsg{Session: name, Create: true}
}

// shown returns the sessions whose names contain the typed text.
func (m *Model) shown() []api.SessionInfo {
	q := strings.ToLower(strings.TrimSpace(m.input))
	if q == "" {
		return m.sessions
	}
	var out []api.SessionInfo
	for _, s := range m.sessions {
		if strings.Contains(strings.ToLower(s.Name), q) {
			out = append(out, s)
		}
	}
	return out
}

func openCmd(o OpenMsg) tea.Cmd { return func() tea.Msg { return o } }

// open opens o, unless its name would be refused: then the notice says why.
func (m *Model) open(o OpenMsg) tea.Cmd {
	if err := (&api.Attach{Session: o.Session}).Validate(); err != nil {
		m.notice = command.Reason(err)
		return nil
	}
	return openCmd(o)
}
