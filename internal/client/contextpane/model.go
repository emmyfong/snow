// Package contextpane draws the context pane: the context occupancy a source
// reports and the conversation's observable context sources and repeated
// reads. The server reads the records and sends each api.ContextState the
// pane draws.
package contextpane

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// Model is the client half of a context pane.
type Model struct {
	id int
	th theme.Theme
	ui.Viewport
	st     api.ContextState
	loaded bool
}

// New returns a context pane for pane id.
func New(id int, th theme.Theme) *Model {
	return &Model{id: id, th: th}
}

// SetSize sets the pane's size in cells.
func (m *Model) SetSize(width, height int) {
	m.Viewport.SetSize(width, height)
	m.Clamp(len(m.lines()))
}

// SetState replaces the pane's state and keeps the view inside it.
func (m *Model) SetState(st api.ContextState) {
	m.st = st
	m.loaded = true
	m.Clamp(len(m.lines()))
}

// SetTheme changes the colors the pane draws with.
func (m *Model) SetTheme(th theme.Theme) {
	if th.Name == m.th.Name {
		return
	}
	m.th = th
}

// Note returns what the status bar says about the pane.
func (m *Model) Note() string { return "" }

// Key handles a key. It returns the message to send to the server, or nil.
func (m *Model) Key(k tea.Key) api.Message {
	if !m.loaded {
		return nil
	}
	if refresh, _ := m.Viewport.Key(k, len(m.lines())); refresh {
		return &api.SidePaneAction{Pane: m.id, Action: api.ContextActRefresh}
	}
	return nil
}

// Click selects nothing; the pane's rows are read, not chosen.
func (m *Model) Click(_, _ int) api.Message { return nil }

// Scroll moves the rows by rows, toward the start when positive.
func (m *Model) Scroll(rows int) { m.ScrollBy(-rows, len(m.lines())) }

// Paste inserts text; the pane takes none.
func (m *Model) Paste(string) {}

// Animating reports whether the pane needs ticks.
func (m *Model) Animating() bool { return false }

// Tick advances the pane.
func (m *Model) Tick() {}

// Cursor says where the terminal's cursor goes; the pane has none.
func (m *Model) Cursor() (x, y int, ok bool) { return 0, 0, false }
