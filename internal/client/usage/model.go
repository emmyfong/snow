// Package usage draws the usage pane: a conversation's observable tokens,
// estimated cost, and per-turn and per-model breakdowns. The server reads
// the conversation's records and sends each api.UsageState the pane draws.
package usage

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// Model is the client half of a usage pane.
type Model struct {
	id     int
	th     theme.Theme
	width  int
	height int
	st     api.UsageState
	loaded bool
	top    int // the first visible body row
}

// New returns a usage pane for pane id.
func New(id int, th theme.Theme) *Model {
	return &Model{id: id, th: th}
}

// SetSize sets the pane's size in cells.
func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height
	m.clampTop()
}

// SetState replaces the pane's state and keeps the view inside it.
func (m *Model) SetState(st api.UsageState) {
	m.st = st
	m.loaded = true
	m.clampTop()
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
	page := max(m.bodyHeight()-1, 1)
	switch {
	case k.Code == tea.KeyUp || k.Text == "k":
		m.scrollBy(-1)
	case k.Code == tea.KeyDown || k.Text == "j":
		m.scrollBy(1)
	case k.Code == tea.KeyPgUp:
		m.scrollBy(-page)
	case k.Code == tea.KeyPgDown:
		m.scrollBy(page)
	case k.Code == tea.KeyHome || k.Text == "g":
		m.top = 0
	case k.Code == tea.KeyEnd || k.Text == "G":
		m.top = len(m.lines())
		m.clampTop()
	case k.Text == "r":
		return &api.SidePaneAction{Pane: m.id, Action: api.UsageActRefresh}
	}
	return nil
}

// Click selects nothing; the pane's rows are read, not chosen.
func (m *Model) Click(_, _ int) api.Message { return nil }

// Scroll moves the rows by rows, toward the start when positive.
func (m *Model) Scroll(rows int) { m.scrollBy(-rows) }

// Paste inserts text; the pane takes none.
func (m *Model) Paste(string) {}

// Animating reports whether the pane needs ticks.
func (m *Model) Animating() bool { return false }

// Tick advances the pane.
func (m *Model) Tick() {}

// Cursor says where the terminal's cursor goes; the pane has none.
func (m *Model) Cursor() (x, y int, ok bool) { return 0, 0, false }

// scrollBy moves the first visible row by rows.
func (m *Model) scrollBy(rows int) {
	m.top += rows
	m.clampTop()
}

// clampTop keeps the first visible row inside the pane.
func (m *Model) clampTop() {
	m.top = min(max(m.top, 0), max(len(m.lines())-m.bodyHeight(), 0))
}

// bodyHeight is the rows the body shows, below the header and above the
// hints.
func (m *Model) bodyHeight() int {
	return max(m.height-2, 0)
}
