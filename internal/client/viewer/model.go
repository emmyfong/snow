// Package viewer draws the read-only file viewer: a bounded file with line
// numbers, syntax colors from Snow's theme, and marks for the lines the
// conversation's recorded changes still explain.
package viewer

import (
	"fmt"
	"hash/fnv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// Model is the client half of a viewer pane.
type Model struct {
	id     int
	th     theme.Theme
	width  int
	height int
	st     api.ViewerState
	loaded bool
	top    int
	note   string

	styled []string // styled text for each viewer line, by index
	hlKey  string   // what styled was built from
	seen   string   // the last path and reveal the model scrolled to
}

// New returns a viewer pane for pane id.
func New(id int, th theme.Theme) *Model {
	return &Model{id: id, th: th}
}

// SetSize sets the pane's size in cells.
func (m *Model) SetSize(width, height int) {
	m.width, m.height = width, height
	m.clampTop()
}

// SetTheme sets the theme.
func (m *Model) SetTheme(th theme.Theme) {
	if th.Name != m.th.Name {
		m.th = th
		m.highlight()
	}
}

// SetState shows st.
func (m *Model) SetState(st api.ViewerState) {
	m.st = st
	m.loaded = true
	m.highlight()
	key := fmt.Sprintf("%s:%d", st.Path, st.Reveal)
	if key != m.seen && st.Reveal > 0 {
		m.seen = key
		m.reveal(st.Reveal)
	}
	m.clampTop()
}

// Note returns a short message for the status bar, or "".
func (m *Model) Note() string { return m.note }

// highlight rebuilds the syntax colors when the text, path, or theme
// changed. Removed lines take no syntax color; the view colors them.
func (m *Model) highlight() {
	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "%s\x00%s\x00", m.st.Path, m.th.Name)
	for _, l := range m.st.Lines {
		_, _ = fmt.Fprintf(h, "%d\x00%s\x00", l.Number, l.Text)
	}
	key := fmt.Sprintf("%x", h.Sum64())
	if key == m.hlKey {
		return
	}
	m.hlKey = key
	m.styled = make([]string, len(m.st.Lines))
	if m.st.Binary || len(m.st.Lines) == 0 {
		return
	}
	var text strings.Builder
	var at []int // the viewer line of each file line
	for i, l := range m.st.Lines {
		if l.Number == 0 {
			continue
		}
		if text.Len() > 0 {
			text.WriteByte('\n')
		}
		text.WriteString(l.Text)
		at = append(at, i)
	}
	parts := tokenLines(m.st.Path, text.String(), m.th)
	for i, idx := range at {
		if i < len(parts) {
			m.styled[idx] = parts[i]
		}
	}
}

// bodyHeight is the number of rows the file text shows.
func (m *Model) bodyHeight() int {
	h := m.height - 2 // header and hints
	if m.noteLine() != "" && m.height >= 4 {
		h--
	}
	return max(h, 0)
}

// reveal scrolls line, one-based, into view.
func (m *Model) reveal(line int) {
	i := line - 1
	m.top = max(i-2, 0)
	m.clampTop()
}

// clampTop keeps the first visible row inside the file.
func (m *Model) clampTop() {
	rows := len(m.st.Lines)
	m.top = min(max(m.top, 0), max(rows-m.bodyHeight(), 0))
}

// row is one drawn row of the body.
type row struct {
	line api.ViewerLine
	text string // syntax-colored text, or ""
}

// bodyRows returns the rows the body shows.
func (m *Model) bodyRows() []row {
	if m.st.Binary || m.st.Error != "" && len(m.st.Lines) == 0 {
		return nil
	}
	out := make([]row, 0, m.bodyHeight())
	for i := m.top; i < len(m.st.Lines) && len(out) < m.bodyHeight(); i++ {
		r := row{line: m.st.Lines[i]}
		if r.line.Number > 0 && i < len(m.styled) {
			r.text = m.styled[i]
		}
		out = append(out, r)
	}
	return out
}

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
		m.clampTop()
	case k.Code == tea.KeyEnd || k.Text == "G":
		m.top = len(m.st.Lines)
		m.clampTop()
	case k.Text == "r":
		return &api.SidePaneAction{Pane: m.id, Action: api.ViewerActRefresh}
	}
	return nil
}

// scrollBy moves the first visible row by rows.
func (m *Model) scrollBy(rows int) {
	m.top += rows
	m.clampTop()
}

// Click selects no line; the viewer's text is read, not chosen.
func (m *Model) Click(_, _ int) api.Message { return nil }

// Scroll moves the text by rows, toward the start when positive.
func (m *Model) Scroll(rows int) { m.scrollBy(-rows) }

// Paste inserts text; the viewer takes none.
func (m *Model) Paste(string) {}

// Animating reports whether the pane needs ticks.
func (m *Model) Animating() bool { return false }

// Tick advances the pane.
func (m *Model) Tick() {}

// Cursor says where the terminal's cursor goes; the viewer has none.
func (m *Model) Cursor() (x, y int, ok bool) { return 0, 0, false }
