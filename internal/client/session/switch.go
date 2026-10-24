package session

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/sessionlist"
)

// SwitchMsg asks the client to leave this session, which keeps running, and
// attach to Session instead, creating it when Create is set. An empty
// Session with Create starts a numbered session.
type SwitchMsg struct {
	Session string
	Create  bool
}

// openSessions shows the session list over the window, with this session
// highlighted.
func (m *Model) openSessions() {
	current := ""
	if m.layout != nil {
		current = m.layout.Session
	}
	m.switcher = sessionlist.New(current, m.now)
	m.switcher.SetSessions(m.sessions)
}

// switcherKey sends a key to the open session list.
func (m *Model) switcherKey(k tea.Key) tea.Cmd {
	return m.switcherResult(m.switcher.Key(k))
}

// switcherMouse handles a mouse event while the list is open: a left click
// chooses or closes; a right click closes; the rest is ignored.
func (m *Model) switcherMouse(msg tea.MouseMsg) tea.Cmd {
	click, ok := msg.(tea.MouseClickMsg)
	if !ok {
		return nil
	}
	if click.Button != tea.MouseLeft {
		m.switcher = nil
		return nil
	}
	return m.switcherResult(m.switcher.Click(m.theme, m.cols, m.windowRows(), click.X, click.Y))
}

// switcherResult closes the list when the user is done with it, and asks
// to switch when another session was chosen.
func (m *Model) switcherResult(r sessionlist.Result) tea.Cmd {
	switch r {
	case sessionlist.Chosen:
		name := m.switcher.Choice()
		m.switcher = nil
		if m.layout != nil && name == m.layout.Session {
			return nil
		}
		return switchTo(SwitchMsg{Session: name})
	case sessionlist.Closed:
		m.switcher = nil
	}
	return nil
}

func switchTo(s SwitchMsg) tea.Cmd { return func() tea.Msg { return s } }

// overlaySwitcher draws the open list over content, the window area of rows
// rows.
func (m *Model) overlaySwitcher(content string, rows int) string {
	if m.switcher == nil {
		return content
	}
	return m.overlayPopup(content, m.switcher, rows)
}
