package chat

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/keys"
	"github.com/emmyfong/snow/pkg/api"
)

// Key handles a key press in the focused chat pane and returns what to
// send to the server, or nil. Enter sends the message; Shift+Enter,
// Alt+Enter, and Ctrl+J add a line, since some terminals cannot tell
// Shift+Enter from Enter. Esc interrupts a running turn. PgUp, PgDn, Up,
// and Down scroll the transcript; any key that edits returns to the
// latest text. Tab and Shift+Tab select a tool cell, and Enter then
// expands or collapses it. While an approval waits, its box takes the
// keys first.
func (m *Model) Key(k tea.Key) *api.ChatInput {
	if p := m.pending(); p != nil {
		if in, ok := m.approvalKey(p, k); ok {
			return in
		}
	}
	switch {
	case m.selected != "" && k.Code == tea.KeyEnter && k.Mod == 0:
		m.expanded[m.selected] = !m.expanded[m.selected]
		m.invalidate()
		m.reveal(m.selected)
		return nil
	case m.selected != "" && k.Code == tea.KeyEscape:
		m.selected = ""
		m.invalidate()
		return nil
	case k.Code == tea.KeyTab && k.Mod == tea.ModShift:
		m.selectTool(1)
		return nil
	case k.Code == tea.KeyTab:
		m.selectTool(-1)
		return nil
	}
	switch {
	case k.Code == tea.KeyEnter && k.Mod&(tea.ModShift|tea.ModAlt) != 0, k.Code == 'j' && k.Mod == tea.ModCtrl:
		m.insert("\n")
	case k.Code == tea.KeyEnter:
		return m.submit()
	case k.Code == tea.KeyEscape:
		if m.snap.Running {
			return m.chatInput(api.InputInterrupt, "")
		}
	case k.Code == tea.KeyPgUp:
		m.Scroll(max(m.transcriptHeight()-1, 1))
	case k.Code == tea.KeyPgDown:
		m.Scroll(-max(m.transcriptHeight()-1, 1))
	case k.Code == tea.KeyUp:
		m.Scroll(1)
	case k.Code == tea.KeyDown:
		m.Scroll(-1)
	case k.Code == tea.KeyLeft:
		m.cursor = max(m.cursor-1, 0)
	case k.Code == tea.KeyRight:
		m.cursor = min(m.cursor+1, len(m.input))
	case k.Code == tea.KeyHome:
		m.cursor = 0
	case k.Code == tea.KeyEnd:
		m.cursor = len(m.input)
		m.Follow()
	case k.Code == tea.KeyBackspace:
		m.unselect()
		if m.cursor > 0 {
			m.input = slices.Delete(m.input, m.cursor-1, m.cursor)
			m.cursor--
		}
		m.Follow()
	case k.Code == tea.KeyDelete:
		if m.cursor < len(m.input) {
			m.input = slices.Delete(m.input, m.cursor, m.cursor+1)
		}
		m.Follow()
	case k.Code == 'u' && k.Mod == tea.ModCtrl:
		m.input, m.cursor = nil, 0
	default:
		if text, ok := keys.Typed(k); ok {
			m.insert(text)
		}
	}
	return nil
}

// Paste adds pasted text where the cursor is. Line breaks stay in the
// message; they do not send it.
func (m *Model) Paste(text string) {
	m.insert(strings.ReplaceAll(text, "\r\n", "\n"))
}

// Retry returns the input that runs the failed turn again.
func (m *Model) Retry() *api.ChatInput { return m.chatInput(api.InputRetry, "") }

func (m *Model) insert(text string) {
	m.unselect()
	r := []rune(text)
	m.input = slices.Insert(m.input, m.cursor, r...)
	m.cursor += len(r)
	m.Follow()
}

// submit sends the typed message, unless it is blank. During a turn the
// server queues it.
func (m *Model) submit() *api.ChatInput {
	text := string(m.input)
	if strings.TrimSpace(text) == "" {
		return nil
	}
	m.input, m.cursor = nil, 0
	m.Follow()
	return m.chatInput(api.InputMessage, text)
}

func (m *Model) chatInput(kind, text string) *api.ChatInput {
	return &api.ChatInput{Pane: m.snap.Pane, Agent: m.snap.Agent, Kind: kind, Text: text}
}

// Scroll moves the transcript by rows: up, toward older text, when rows
// is positive. It stops at the first and the latest row.
func (m *Model) Scroll(rows int) {
	m.scroll = min(max(m.scroll+rows, 0), max(len(m.transcriptRows())-m.transcriptHeight(), 0))
}

// unselect gives the keyboard back to the message being typed.
func (m *Model) unselect() {
	if m.selected != "" {
		m.selected = ""
		m.invalidate()
	}
}

// Follow returns to the latest text, and keeps to it as new text comes.
func (m *Model) Follow() { m.scroll = 0 }
