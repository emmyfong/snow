// Package session is the attached view: the panes of one session. Until
// panes and windows arrive, it shows a single pane that fills the screen.
package session

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// DetachMsg asks the client to detach from the session.
type DetachMsg struct{}

// Model is the session screen. Send queues a message for the server; it must
// keep order.
type Model struct {
	send       func(api.Message)
	pane       int
	lines      []string
	cursor     api.Cursor
	cols, rows int
	prefixed   bool // the prefix key was pressed; the next key is a command
}

// prefix is the key that starts a Snow command, as in tmux. Rebinding comes
// with the config file.
var prefix = tea.Key{Code: 'b', Mod: tea.ModCtrl}

func isPrefix(k tea.Key) bool { return k.Code == prefix.Code && k.Mod == prefix.Mod }

// New returns a session screen that sends input through send.
func New(send func(api.Message)) *Model { return &Model{send: send} }

// SetSize sets the screen size in cells.
func (m *Model) SetSize(cols, rows int) { m.cols, m.rows = cols, rows }

// Update applies server messages and handles keys.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case *api.Layout:
		m.applyLayout(msg)
	case *api.PaneUpdate:
		m.applyUpdate(msg)
	case tea.KeyPressMsg:
		return m.key(msg.Key())
	case tea.PasteMsg:
		if m.pane != 0 {
			m.send(&api.Input{Pane: m.pane, Paste: msg.Content})
		}
	}
	return nil
}

// key handles one key press: the prefix, a command after the prefix, or a
// key for the pane.
func (m *Model) key(k tea.Key) tea.Cmd {
	if m.prefixed {
		m.prefixed = false
		switch {
		case isPrefix(k):
			m.sendKey(k) // prefix twice types the prefix key itself
		case k.Code == 'd' && k.Mod == 0:
			return func() tea.Msg { return DetachMsg{} }
		}
		return nil
	}
	if isPrefix(k) {
		m.prefixed = true
		return nil
	}
	m.sendKey(k)
	return nil
}

func (m *Model) sendKey(k tea.Key) {
	if m.pane == 0 {
		return
	}
	if key, ok := toAPIKey(k); ok {
		m.send(&api.Input{Pane: m.pane, Key: &key})
	}
}

func (m *Model) applyLayout(l *api.Layout) {
	for _, w := range l.Windows {
		if w.Index != l.Active || len(w.Panes) == 0 {
			continue
		}
		p := w.Panes[0]
		if p.ID != m.pane {
			m.pane, m.lines = p.ID, nil
		}
		m.lines = resize(m.lines, p.Rect.H)
	}
}

func (m *Model) applyUpdate(u *api.PaneUpdate) {
	if u.Pane != m.pane {
		return
	}
	for _, l := range u.Lines {
		if l.Row < 0 {
			continue
		}
		if l.Row >= len(m.lines) {
			m.lines = resize(m.lines, l.Row+1)
		}
		m.lines[l.Row] = l.Text
	}
	if u.Cursor != nil {
		m.cursor = *u.Cursor
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

// View draws the pane over the whole screen. A client larger than the
// session shows blank space beyond the pane.
func (m *Model) View() tea.View {
	v := tea.NewView(ui.PaneView(m.lines, m.cols, m.rows))
	if m.cursor.Visible && m.cursor.X < m.cols && m.cursor.Y < m.rows {
		v.Cursor = tea.NewCursor(m.cursor.X, m.cursor.Y)
	}
	return v
}
