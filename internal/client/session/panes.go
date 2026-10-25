package session

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// ownPane is a pane the client draws itself from server messages, such as
// a chat, rather than a terminal screen the server draws. The session
// creates one for each pane whose PaneInfo.Kind it knows, and talks to it
// only through this interface, so a kind can be redesigned without
// editing the session.
type ownPane interface {
	SetSize(width, height int)
	SetTheme(th theme.Theme)
	// View returns the pane's rows; Cursor says where the terminal's
	// cursor goes in it, if anywhere.
	View() []string
	Cursor() (x, y int, ok bool)
	// Feed applies a server message meant for the pane.
	Feed(msg api.Message)
	// Key and Click handle input. They return what to send to the server,
	// or nil, and a note for the status bar, or "".
	Key(k tea.Key) (api.Message, string)
	Click(x, y int) (api.Message, string)
	Paste(text string)
	// Scroll moves the pane's content by rows, toward older content when
	// positive.
	Scroll(rows int)
	// Animating reports whether the pane wants Tick calls, for example
	// for a spinner.
	Animating() bool
	Tick()
}

// newOwnPane returns the pane for p's kind, or nil for a terminal pane or
// a kind this client does not know.
func (m *Model) newOwnPane(kind string, id int) ownPane {
	switch kind {
	case api.PaneChat:
		return newChatPane(id, m.theme, m.now, !m.hideUsage)
	}
	return nil
}

// ownTickMsg advances every animating pane.
type ownTickMsg struct{}

// ownTickEvery is how often animating panes are redrawn.
const ownTickEvery = 120 * time.Millisecond

// wheelRows is how far one wheel notch scrolls a pane the client draws.
const wheelRows = 3

// ownMsg handles a message for the panes the client draws: a chat
// message from the server, or a tick. A chat message for a pane no layout
// has listed yet starts its chat pane; the next Layout sizes it.
func (m *Model) ownMsg(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case *api.ChatSnapshot:
		m.feed(api.PaneChat, msg.Pane, msg)
	case *api.ChatEvent:
		m.feed(api.PaneChat, msg.Pane, msg)
	case ownTickMsg:
		m.ownTicking = false
		for _, p := range m.own {
			if p.Animating() {
				p.Tick()
			}
		}
	}
	return m.ownTick()
}

func (m *Model) feed(kind string, id int, msg api.Message) {
	p := m.own[id]
	if p == nil {
		if p = m.newOwnPane(kind, id); p == nil {
			return
		}
		m.own[id] = p
		m.syncPanes()
	}
	p.Feed(msg)
}

// ownTick keeps one tick pending while a pane animates.
func (m *Model) ownTick() tea.Cmd {
	if m.ownTicking {
		return nil
	}
	for _, p := range m.own {
		if p.Animating() {
			m.ownTicking = true
			return tea.Tick(ownTickEvery, func(time.Time) tea.Msg { return ownTickMsg{} })
		}
	}
	return nil
}

// syncPanes creates a pane for each listed pane of a kind the client
// draws, gives each its size, and forgets the ones whose panes are gone. A
// zoomed window lists only its zoomed pane, so while any window is zoomed
// unlisted panes are kept: they come back when it unzooms.
func (m *Model) syncPanes() {
	if m.layout == nil {
		return
	}
	listed, zoomed := map[int]bool{}, false
	for _, w := range m.layout.Windows {
		zoomed = zoomed || w.Zoomed
		for _, p := range w.Panes {
			listed[p.ID] = true
			if m.own[p.ID] == nil {
				if o := m.newOwnPane(p.Kind, p.ID); o != nil {
					m.own[p.ID] = o
				}
			}
			if o := m.own[p.ID]; o != nil {
				o.SetSize(p.Rect.W, p.Rect.H)
			}
		}
	}
	if zoomed {
		return
	}
	for id := range m.own {
		if !listed[id] {
			delete(m.own, id)
		}
	}
}

// focusedOwn returns the focused pane when the client draws it, or nil
// when it is a terminal.
func (m *Model) focusedOwn() ownPane {
	if p := m.focused(); p != nil {
		return m.own[p.ID]
	}
	return nil
}

// ownKey gives k to the focused pane, which the client must draw, sends
// what it returns, and shows its note.
func (m *Model) ownKey(k tea.Key) tea.Cmd {
	return m.ownResult(m.focusedOwn().Key(k))
}

func (m *Model) ownResult(msg api.Message, note string) tea.Cmd {
	if msg != nil {
		m.send(msg)
	}
	if note != "" {
		return m.ShowError(note)
	}
	return nil
}

// newChat splits the focused pane with a chat on the right.
func (m *Model) newChat() {
	m.paneCommand(api.OpSplit, api.ArgDir, api.DirRight, api.ArgKind, api.PaneChat)
}
