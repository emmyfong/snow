package session

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/chat"
	"github.com/emmyfong/snow/pkg/api"
)

// chatTickMsg turns the spinner of every running chat.
type chatTickMsg struct{}

// chatTickEvery is how often a running chat's spinner and elapsed time
// are redrawn.
const chatTickEvery = 120 * time.Millisecond

// wheelRows is how far one wheel notch scrolls a chat.
const wheelRows = 3

// chatMsg applies a chat message from the server. A snapshot for a pane
// the screen does not know yet starts its chat; the next Layout sizes it.
func (m *Model) chatMsg(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case *api.ChatSnapshot:
		if c := m.chats[msg.Pane]; c != nil {
			c.Snapshot(msg)
		} else {
			c = chat.New(msg, m.theme, m.now)
			c.SetUsage(!m.hideUsage)
			m.chats[msg.Pane] = c
			m.sizeChats()
		}
	case *api.ChatEvent:
		if c := m.chats[msg.Pane]; c != nil {
			c.Event(msg)
		}
	case chatTickMsg:
		m.chatTicking = false
		for _, c := range m.chats {
			if c.Running() {
				c.Tick()
			}
		}
	}
	return m.chatTick()
}

// chatTick keeps one spinner tick pending while a chat runs.
func (m *Model) chatTick() tea.Cmd {
	if m.chatTicking {
		return nil
	}
	for _, c := range m.chats {
		if c.Running() {
			m.chatTicking = true
			return tea.Tick(chatTickEvery, func(time.Time) tea.Msg { return chatTickMsg{} })
		}
	}
	return nil
}

// sizeChats gives each chat its pane's size and forgets chats whose panes
// are gone.
func (m *Model) sizeChats() {
	if m.layout == nil {
		return
	}
	present := map[int]bool{}
	for _, w := range m.layout.Windows {
		for _, p := range w.Panes {
			present[p.ID] = true
			if c := m.chats[p.ID]; c != nil {
				c.SetSize(p.Rect.W, p.Rect.H)
			}
		}
	}
	for id := range m.chats {
		if !present[id] {
			delete(m.chats, id)
		}
	}
}

// focusedChat returns the chat of the focused pane, or nil when the
// focused pane is a terminal.
func (m *Model) focusedChat() *chat.Model {
	if p := m.focused(); p != nil {
		return m.chats[p.ID]
	}
	return nil
}

// chatKey gives k to the focused chat, which must exist, and sends what it
// returns.
func (m *Model) chatKey(k tea.Key) tea.Cmd {
	c := m.focusedChat()
	if in := c.Key(k); in != nil {
		m.send(in)
	}
	return m.chatAsked(c)
}

// chatAsked shows what to do for a request c raised.
func (m *Model) chatAsked(c *chat.Model) tea.Cmd {
	switch c.Asked() {
	case chat.AskLogin:
		// TODO(#114): open /login instead.
		return m.ShowError("Run snow auth in a terminal to save a key, then retry.")
	case chat.AskPickModel:
		// TODO(#113): open the model picker instead.
		return m.ShowError("Set [chat] model in config.toml, then open a new chat.")
	}
	return nil
}

// newChat splits the focused pane with a chat on the right.
func (m *Model) newChat() {
	m.paneCommand(api.OpSplit, api.ArgDir, api.DirRight, api.ArgKind, api.PaneChat)
}
