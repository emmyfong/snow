// Package home is the home page: Snow's wordmark, an input box to open or
// create a session, the running sessions, and key hints. It is the screen
// plain `snow` opens, and it can be replaced without touching other screens.
package home

import (
	"cmp"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
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
	selected   int    // index into the shown sessions; -1 means none
	back       string // the session Esc returns to; "" means Esc quits
	cols, rows int
}

// New returns the home page for Snow version, drawn with th.
func New(version string, th theme.Theme) *Model {
	return &Model{version: version, theme: th, base: th, now: time.Now, selected: -1}
}

// SetBack makes Esc return to session name instead of quitting, for a home
// page opened from a session to switch away from it.
func (m *Model) SetBack(name string) { m.back = name }

// SetSize sets the screen size in cells.
func (m *Model) SetSize(cols, rows int) { m.cols, m.rows = cols, rows }

// Update handles the session list, keys, and clicks.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.ColorProfileMsg:
		m.theme = m.base.For(msg.Profile)
	case *api.SessionList:
		m.sessions = slices.Clone(msg.Sessions)
		slices.SortFunc(m.sessions, func(a, b api.SessionInfo) int {
			return cmp.Or(b.LastUsed.Compare(a.LastUsed), cmp.Compare(a.Name, b.Name))
		})
		m.selected = min(m.selected, len(m.shown())-1)
	case tea.KeyPressMsg:
		return m.key(msg.Key())
	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			if i, ok := m.layout().rowAt(msg.Y); ok {
				return openCmd(OpenMsg{Session: m.shown()[i].Name})
			}
		}
	}
	return nil
}

func (m *Model) key(k tea.Key) tea.Cmd {
	switch {
	case k.Code == 'c' && k.Mod == tea.ModCtrl:
		return func() tea.Msg { return QuitMsg{} }
	case k.Code == tea.KeyEscape:
		if m.input == "" && m.back != "" {
			return openCmd(OpenMsg{Session: m.back})
		}
		if m.input == "" {
			return func() tea.Msg { return QuitMsg{} }
		}
		m.input, m.selected = "", -1
	case k.Code == tea.KeyEnter:
		return openCmd(m.choice())
	case k.Code == tea.KeyUp:
		if m.selected > 0 {
			m.selected--
		}
	case k.Code == tea.KeyDown:
		if m.selected < len(m.shown())-1 {
			m.selected++
		}
	case k.Code == tea.KeyBackspace:
		if r := []rune(m.input); len(r) > 0 {
			m.input, m.selected = string(r[:len(r)-1]), -1
		}
	case k.Text != "" && k.Mod&(tea.ModCtrl|tea.ModAlt) == 0:
		m.input, m.selected = m.input+k.Text, -1
	}
	return nil
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
