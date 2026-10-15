// Package session is the attached view: the panes of the shown window, their
// borders, and the status bar.
package session

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// DetachMsg asks the client to detach from the session.
type DetachMsg struct{}

// Model is the session screen. Send queues a message for the server; it must
// keep order.
type Model struct {
	send       func(api.Message)
	theme      theme.Theme
	now        func() time.Time
	layout     *api.Layout
	panes      map[int]*paneState
	profiles   api.ProfileList
	cols, rows int
	prefixed   bool        // the prefix key was pressed; the next key is a command
	early      []api.Input // input typed before the first Layout named a pane
	message    string      // why the last request was refused
	messageAt  time.Time
	ticking    bool // the clock tick is running
	picker     *picker.Model
	confirm    *confirmation // a question waiting for y or n
	pending    api.Command   // the command the open picker completes
}

// paneState is what the screen knows of one pane: its rows and its cursor.
type paneState struct {
	lines  []string
	cursor api.Cursor
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

type clockMsg struct{}

// New returns a session screen that sends input through send.
func New(send func(api.Message)) *Model {
	return &Model{send: send, theme: theme.Default(), now: time.Now, panes: map[int]*paneState{}}
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
	case *api.ProfileList:
		m.profiles = *msg
	case *api.Error:
		m.message, m.messageAt = msg.Message, m.now()
		at := m.messageAt
		return tea.Tick(messageFor, func(time.Time) tea.Msg { return clearMessageMsg{at} })
	case clearMessageMsg:
		if msg.at.Equal(m.messageAt) {
			m.message = ""
		}
	case clockMsg:
		return tick()
	case tea.KeyPressMsg:
		return m.key(msg.Key())
	case tea.PasteMsg:
		if m.picker == nil {
			m.input(api.Input{Paste: msg.Content})
		}
	}
	return nil
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
