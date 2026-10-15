// Package picker is the profile picker: the popup that asks which program a
// new pane or window runs. The current pane's profile is preselected, so
// Enter keeps tmux's speed.
package picker

import (
	"slices"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
)

// Result is what a key press did to the picker.
type Result int

// Key results.
const (
	Open      Result = iota // the picker stays open
	Accepted                // Choice holds the profile
	Cancelled               // the user closed the picker
)

// Model is one open picker.
type Model struct {
	title    string
	profiles []string
	current  string
	folder   string
	selected int
}

// New opens a picker titled title over profiles. Current is the profile of
// the pane the action starts from; it is preselected and labeled with
// folder, where the new pane starts.
func New(title string, profiles []string, current, folder string) *Model {
	if current != "" && !slices.Contains(profiles, current) {
		profiles = append([]string{current}, profiles...)
	}
	return &Model{title: title, profiles: profiles, current: current, folder: folder,
		selected: max(slices.Index(profiles, current), 0)}
}

// Key handles one key press.
func (m *Model) Key(k tea.Key) Result {
	n := len(m.profiles)
	switch {
	case k.Code == tea.KeyEscape:
		return Cancelled
	case k.Code == tea.KeyEnter:
		if n == 0 {
			return Cancelled
		}
		return Accepted
	case n == 0:
	case k.Code == tea.KeyUp || k.Text == "k":
		m.selected = (m.selected - 1 + n) % n
	case k.Code == tea.KeyDown || k.Text == "j":
		m.selected = (m.selected + 1) % n
	}
	return Open
}

// Choice returns the selected profile.
func (m *Model) Choice() string {
	if m.selected < len(m.profiles) {
		return m.profiles[m.selected]
	}
	return ""
}

// Height returns the picker's height in rows.
func (m *Model) Height() int { return len(m.profiles) + 2 }

// View draws the picker width cells wide.
func (m *Model) View(th theme.Theme, width int) string {
	items := make([]string, len(m.profiles))
	for i, p := range m.profiles {
		items[i] = p
		if p == m.current {
			items[i] += "  " + m.folder + " (current)"
		}
	}
	return ui.Popup(th, ui.PopupOptions{Title: m.title, Items: items, Selected: m.selected, Width: width})
}
