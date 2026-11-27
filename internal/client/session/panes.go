package session

import (
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/chat"
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
	var pick tea.Cmd
	switch msg := msg.(type) {
	case *api.ChatSnapshot:
		m.feed(api.PaneChat, msg.Pane, msg)
		if m.pendingRuntime != "" && (m.pendingRuntimePane == 0 || m.pendingRuntimePane == msg.Pane) {
			connection := m.pendingRuntime
			m.pendingRuntime, m.pendingRuntimePane = "", 0
			m.startRuntimeBinding(msg.Pane, msg.Agent, connection)
		} else if !m.pendingShell && m.shellStart == 0 {
			pick = m.maybeStartChat(msg.Pane)
		}
		m.sendListRequest(msg.Pane)
		m.sendFileRequest(msg.Pane)
	case *api.ChatEvent:
		m.feed(api.PaneChat, msg.Pane, msg)
		m.sendListRequest(msg.Pane)
	case *api.FileList:
		m.feed(api.PaneChat, msg.Pane, msg)
	case *api.SidePaneState:
		m.feed(msg.Kind, msg.Pane, msg)
	case ownTickMsg:
		m.ownTicking = false
		for _, p := range m.own {
			if p.Animating() {
				p.Tick()
			}
		}
	}
	return tea.Batch(pick, m.ownTick())
}

func (m *Model) applyStartupShellLayout() {
	if !m.pendingShell && m.shellStart == 0 {
		return
	}
	w := m.shown()
	if w == nil || len(w.Panes) == 0 {
		return
	}
	switch m.shellStart {
	case 0:
		var chatPane *api.PaneInfo
		for i := range w.Panes {
			pane := &w.Panes[i]
			if pane.Kind == api.PaneChat && (chatPane == nil || pane.Focused) {
				chatPane = pane
				if pane.Focused {
					break
				}
			}
		}
		if chatPane == nil {
			m.pendingShell = false
			m.flushEarly()
			return
		}
		m.shellChat = chatPane.ID
		m.shellBefore = make(map[int]bool, len(w.Panes))
		for _, pane := range w.Panes {
			m.shellBefore[pane.ID] = true
		}
		m.shellStart = 1
		m.send(&api.Command{Op: api.OpSplit, Target: chatPane.ID, Args: map[string]string{api.ArgKind: api.PaneTerminal, api.ArgDir: api.DirRight}})
	case 1:
		for _, pane := range w.Panes {
			if !m.shellBefore[pane.ID] && pane.Kind == api.PaneTerminal {
				m.shellTerminal = pane.ID
				m.shellStart = 2
				m.send(&api.Command{Op: api.OpClose, Target: m.shellChat})
				return
			}
		}
	case 2:
		chatPresent, terminalPresent := false, false
		for _, pane := range w.Panes {
			chatPresent = chatPresent || pane.ID == m.shellChat
			terminalPresent = terminalPresent || pane.ID == m.shellTerminal && pane.Kind == api.PaneTerminal
		}
		if chatPresent {
			return
		}
		for _, pane := range w.Panes {
			if pane.Kind == api.PaneChat {
				m.shellChat = pane.ID
				m.send(&api.Command{Op: api.OpClose, Target: pane.ID})
				return
			}
		}
		if terminalPresent {
			m.pendingShell, m.shellStart, m.shellBefore = false, 0, nil
			if m.preset != "" {
				m.send(&api.Command{Op: api.OpLayout, Target: m.shellTerminal, Args: map[string]string{api.ArgName: m.preset}})
				m.preset = ""
			}
			m.flushEarly()
		}
	}
}

func (m *Model) flushEarly() {
	if m.focused() == nil {
		return
	}
	early := m.early
	m.early = nil
	for _, input := range early {
		m.input(input)
	}
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
	focusedID := m.focusedPaneID()
	for id, pane := range m.own {
		if f, ok := pane.(interface{ SetFocused(bool) }); ok {
			f.SetFocused(id == focusedID)
		}
	}
	listed, zoomed := map[int]bool{}, false
	for _, w := range m.layout.Windows {
		zoomed = zoomed || w.Zoomed
		for _, p := range w.Panes {
			listed[p.ID] = true
			if m.own[p.ID] == nil {
				if o := m.newOwnPane(p.Kind, p.ID); o != nil {
					m.own[p.ID] = o
					if r, ok := o.(providerReady); ok {
						r.SetProviderReady(anyConnected(m.authList))
					}
				}
			}
			if o := m.own[p.ID]; o != nil {
				o.SetSize(p.Rect.W, p.Rect.H)
				if f, ok := o.(interface{ SetFocused(bool) }); ok {
					f.SetFocused(p.ID == focusedID)
				}
			}
		}
	}
	if zoomed {
		return
	}
	for id := range m.own {
		if !listed[id] {
			delete(m.own, id)
			if m.ownPress != nil && m.ownPress.pane == id {
				m.ownPress = nil
			}
			delete(m.firstRun, id)
			delete(m.authWait, id)
			delete(m.authAfter, id)
			if m.authPick != nil && m.authPane == id {
				m.authPick, m.authPane = nil, 0
			}
			if m.over != nil && m.over.pane == id {
				m.over, m.resume = nil, nil
			}
			if m.logoutPane == id {
				m.logoutPane = 0
			}
			if m.loginPane == id {
				m.loginPane = 0
			}
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
	p := m.focused()
	if p == nil {
		return nil
	}
	o := m.own[p.ID]
	if o == nil {
		return nil
	}
	if c, ok := o.(chatPane); ok {
		c.SetSubmitBlocked(m.runtimeBindFailed && m.runtimeBindingPane == p.ID)
	}
	msg, note := o.Key(k)
	return m.ownResult(p.ID, o, msg, note)
}

// asker is an own pane that may ask the session to open an overlay, such as
// the model picker.
type asker interface{ Asked() string }

// lister is an own pane that asks for the server's list of past
// conversations, such as an empty chat. It returns the request to send, once.
type lister interface {
	ListRequest() *api.ConversationListRequest
}

// sendListRequest sends the list request pane id has pending, if any.
func (m *Model) sendListRequest(id int) {
	if l, ok := m.own[id].(lister); ok {
		if r := l.ListRequest(); r != nil {
			m.send(r)
		}
	}
}

// fileLister is an own pane that asks for the files of its folder, such as
// a chat's @ picker and its links.
type fileLister interface {
	FileRequest() *api.FileListRequest
}

// sendFileRequest sends the file request pane id has pending, if any.
func (m *Model) sendFileRequest(id int) {
	if l, ok := m.own[id].(fileLister); ok {
		if r := l.FileRequest(); r != nil {
			m.send(r)
		}
	}
}

// fileOpener is an own pane that asks to open a file in the viewer; see
// AskedFile.
type fileOpener interface {
	AskedFile() (string, int)
}

// openAskedFile sends the file open pane id asked for, if any.
func (m *Model) openAskedFile(id int) {
	f, ok := m.own[id].(fileOpener)
	if !ok {
		return
	}
	path, line := f.AskedFile()
	if path == "" {
		return
	}
	args := map[string]string{api.ArgPath: path}
	if line > 0 {
		args[api.ArgLine] = strconv.Itoa(line)
	}
	m.send(&api.Command{Op: api.OpOpenFile, Target: id, Args: args})
}

// providerReady is an own pane that says whether any provider has a key,
// such as a chat's empty state.
type providerReady interface{ SetProviderReady(bool) }

// ownResult sends what pane id returned, opens the overlay it asked for, and
// shows its note.
func (m *Model) ownResult(id int, p ownPane, msg api.Message, note string) tea.Cmd {
	if m.startupInputBlocked(id) {
		switch msg.(type) {
		case *api.ChatInput, *api.Input:
			return nil
		}
	}
	if msg != nil {
		m.send(msg)
	}
	m.sendListRequest(id)
	m.sendFileRequest(id)
	m.openAskedFile(id)
	if a, ok := p.(asker); ok {
		switch a.Asked() {
		case chat.AskRunControls:
			return m.openRuntimeControls(id)
		case chat.AskPickModel:
			return m.openModelPicker(id)
		case chat.AskLogin:
			return m.openLoginPicker(id)
		case chat.AskLogout:
			return m.askLogout(id)
		case chat.AskResume:
			return m.openResume(id)
		case chat.AskPickMode:
			return m.openModePicker(id)
		case chat.AskHelp:
			return m.openHelp(id)
		case chat.AskAgentNew:
			return m.openAgentLaunch(id)
		case chat.AskAgents:
			return m.openBoard()
		case chat.AskOpen:
			if kind := askedArg(p); kind != "" {
				m.openKind(id, kind)
				return nil
			}
			return m.openKindPicker(id)
		case chat.AskLayout:
			if name := askedArg(p); name != "" {
				m.arrange(id, name)
				return nil
			}
			return m.openLayoutPicker(id)
		}
	}
	if note != "" {
		return m.ShowError(note)
	}
	return nil
}

// newClaude splits the focused pane with a Claude Code pane on the right,
// in its folder. The server picks the Claude Code profile that fits the
// pane, for example the one inside the same WSL distribution.
func (m *Model) newClaude() {
	args := []string{api.ArgDir, api.DirRight, api.ArgAgent, api.AgentClaude}
	if p := m.focused(); p != nil && p.Folder != "" {
		args = append(args, api.ArgFolder, p.Folder)
	}
	m.paneCommand(api.OpSplit, args...)
}

// newChat splits the focused pane with a chat on the right.
func (m *Model) newChat() {
	m.paneCommand(api.OpSplit, api.ArgDir, api.DirRight, api.ArgKind, api.PaneChat)
}
