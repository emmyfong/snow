package session

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/internal/client/whichkey"
)

// hintDelay is how long the screen waits after the prefix before it shows
// the which-key popup. A key typed sooner never shows it.
const hintDelay = 500 * time.Millisecond

// hintMsg fires hintDelay after a prefix press. Seq names that press, so a
// timer that outlived its press is ignored.
type hintMsg struct{ seq int }

// startPrefix waits for the next key and starts the hint timer.
func (m *Model) startPrefix() tea.Cmd {
	m.prefixed = true
	m.hint = false
	m.prefixSeq++
	seq := m.prefixSeq
	return tea.Tick(m.hintDelay, func(time.Time) tea.Msg { return hintMsg{seq} })
}

// endPrefix stops waiting for a key and hides the popup.
func (m *Model) endPrefix() {
	m.prefixed = false
	m.hint = false
}

func (m *Model) showHint(h hintMsg) {
	if m.prefixed && h.seq == m.prefixSeq {
		m.hint = true
	}
}

// overlayHint draws the popup centered over content, the window area of rows
// rows.
func (m *Model) overlayHint(content string, rows int) string {
	if !m.hint {
		return content
	}
	box := whichkey.View(m.theme, whichkey.Options{
		Prefix: m.keys.PrefixLabel(), Entries: m.keys.Entries(), MaxWidth: m.cols - 4, MaxHeight: rows,
	})
	if box == "" {
		return content
	}
	w, h := ui.Size(box)
	return ui.Overlay(content, box, max((m.cols-w)/2, 0), max((rows-h)/2, 0))
}
