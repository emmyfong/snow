package session

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// openPicker asks which profile cmd should start. Every split and new
// window opens it; the focused pane's profile and folder are the defaults.
func (m *Model) openPicker(title string, cmd api.Command) {
	p := m.focused()
	if p == nil {
		return
	}
	if cmd.Op == api.OpSplit {
		cmd.Target = p.ID
	}
	m.pending = cmd
	m.picker = picker.New(title, m.profiles.Profiles, p.Profile, p.Folder)
}

// pickerKey sends a key to the open picker and, when it is accepted, sends
// the pending command with the chosen profile.
func (m *Model) pickerKey(k tea.Key) {
	switch m.picker.Key(k) {
	case picker.Accepted:
		cmd := m.pending
		cmd.Args[api.ArgProfile] = m.picker.Choice()
		if p := m.focused(); p != nil && p.Folder != "" {
			cmd.Args[api.ArgFolder] = p.Folder
		}
		m.send(&cmd)
		m.picker = nil
	case picker.Cancelled:
		m.picker = nil
	}
}

// overlayPicker draws the open picker centered over content, which is the
// window area of rows rows.
func (m *Model) overlayPicker(content string, rows int) string {
	if m.picker == nil {
		return content
	}
	width := min(56, m.cols-4)
	box := m.picker.View(m.theme, width)
	if box == "" {
		return content
	}
	x := max((m.cols-width)/2, 0)
	y := max((rows-m.picker.Height())/2, 0)
	return ui.Overlay(content, box, x, y)
}
