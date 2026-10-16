package home

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// placement is where the page's parts go for the current size, shared by
// drawing and by clicks.
type placement struct {
	header         bool // wordmark and version fit
	top            int  // first row of the page
	x, width       int  // the column and width of the input, list, and hints
	listTop, lines int  // the first row of the session list and its rows
	first          int  // index of the first session shown
	count          int  // sessions shown
}

// Rows of each part. The header is the wordmark, the version, and a blank.
const (
	headerRows = ui.WordmarkHeight + 2
	inputRows  = 3                         // a framed one-line input
	fixedRows  = inputRows + 1 + 1 + 1 + 1 // input, blank, label, blank, hints
)

// layout places the page in the screen: centered, the list as tall as the
// room allows, the wordmark dropped first when the screen is short.
func (m *Model) layout() placement {
	p := placement{width: min(64, max(m.cols-4, 0))}
	p.x = max((m.cols-p.width)/2, 0)
	shown, selected := m.listLen(), m.listSel()
	room := m.rows - fixedRows
	p.header = m.cols >= ui.WordmarkWidth && room-headerRows >= min(max(shown, 1), 3)
	if p.header {
		room -= headerRows
	}
	p.lines = min(max(shown, 1), max(room, 1))
	total := fixedRows + p.lines
	if p.header {
		total += headerRows
	}
	p.top = max((m.rows-total)/2, 0)
	p.listTop = p.top + inputRows + 2
	if p.header {
		p.listTop += headerRows
	}
	p.count = min(shown, p.lines)
	if selected >= p.count {
		p.first = selected - p.count + 1
	}
	return p
}

// listLen and listSel are the list's length and selected row: the matching
// commands while a slash command is typed, else the sessions.
func (m *Model) listLen() int {
	if m.commanding() {
		return len(m.matches())
	}
	return len(m.shown())
}

func (m *Model) listSel() int {
	if m.commanding() {
		return m.cmdSel
	}
	return m.selected
}

// rowAt returns the index of the shown session on screen row y.
func (p placement) rowAt(y int) (int, bool) {
	if y < p.listTop || y >= p.listTop+p.count {
		return 0, false
	}
	return p.first + y - p.listTop, true
}

// View draws the home page.
func (m *Model) View() tea.View {
	th := m.theme
	p := m.layout()
	muted := lipgloss.NewStyle().Foreground(th.Muted)
	lines := make([]string, 0, m.rows)
	for range p.top {
		lines = append(lines, "")
	}
	if p.header {
		lines = append(lines, strings.Split(ui.Wordmark(th, m.cols), "\n")...)
		lines = append(lines, center(muted.Render(m.version), m.cols), "")
	}
	field := ui.Input(th, ui.InputOptions{
		Value: m.input, Placeholder: "session name, Enter for a new one, / for commands",
		Prompt: th.Glyphs.Pointer, Width: max(p.width-2, 0), Focused: true,
	})
	lines = append(lines, indent(strings.Split(ui.Panel(th, ui.PanelOptions{Body: field, Width: p.width}), "\n"), p.x)...)
	notice := ""
	if m.notice != "" {
		notice = indent([]string{lipgloss.NewStyle().Foreground(th.Error).Render(ansi.Truncate(m.notice, p.width, "…"))}, p.x)[0]
	}
	lines = append(lines, notice)
	if m.commanding() {
		lines = append(lines, m.commandList(p)...)
	} else {
		lines = append(lines, m.sessionList(p)...)
	}
	lines = append(lines, "", indent([]string{ui.KeyHints(th, m.hints(), p.width)}, p.x)[0])
	for len(lines) < m.rows {
		lines = append(lines, "")
	}
	for i, l := range lines[:m.rows] {
		lines[i] = ansi.Truncate(l, m.cols, "") + strings.Repeat(" ", max(m.cols-ansi.StringWidth(l), 0))
	}
	content := strings.Join(lines[:m.rows], "\n")
	switch {
	case m.confirm != nil:
		box, x, y := m.confirm.View(th, m.cols, m.rows)
		content = ui.Overlay(content, box, x, y)
	case m.help:
		box, x, y := m.helpPanel()
		content = ui.Overlay(content, box, x, y)
	}
	v := tea.NewView(content)
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// sessionList draws the label and the sessions that fit.
func (m *Model) sessionList(p placement) []string {
	th := m.theme
	muted := lipgloss.NewStyle().Foreground(th.Muted)
	lines := []string{indent([]string{muted.Render("Sessions")}, p.x)[0]}
	if p.count == 0 {
		lines = append(lines, indent([]string{muted.Render("No sessions yet. Type a name and press Enter.")}, p.x)[0])
	} else {
		shown := m.shown()[p.first : p.first+p.count]
		items := make([]ui.ListItem, len(shown))
		for i, s := range shown {
			items[i] = ui.ListItem{Cols: []string{s.Name, state(s), ago(m.now().Sub(s.LastUsed))}}
		}
		list := ui.List(th, ui.ListOptions{Items: items, Selected: m.selected - p.first, Width: p.width})
		lines = append(lines, indent(strings.Split(list, "\n"), p.x)...)
	}
	return lines
}

// commandList draws the label and the matching commands that fit.
func (m *Model) commandList(p placement) []string {
	th := m.theme
	muted := lipgloss.NewStyle().Foreground(th.Muted)
	lines := []string{indent([]string{muted.Render("Commands")}, p.x)[0]}
	ms := m.matches()
	if p.count == 0 {
		return append(lines, indent([]string{muted.Render("No command matches. Type /help to see them all.")}, p.x)[0])
	}
	items := make([]ui.ListItem, 0, p.count)
	for _, c := range ms[p.first : p.first+p.count] {
		items = append(items, ui.ListItem{Cols: []string{strings.TrimSpace("/" + c.name + " " + c.args), c.desc}})
	}
	list := ui.List(th, ui.ListOptions{Items: items, Selected: m.cmdSel - p.first, Width: p.width})
	return append(lines, indent(strings.Split(list, "\n"), p.x)...)
}

// hints are the keys the page answers to now.
func (m *Model) hints() []ui.Hint {
	if m.commanding() {
		return []ui.Hint{{Key: "enter", Desc: "run"}, {Key: "tab", Desc: "complete"}, {Key: "↑↓", Desc: "select"}, {Key: "esc", Desc: "clear"}}
	}
	return []ui.Hint{{Key: "enter", Desc: "open or create"}, {Key: "/", Desc: "commands"}, {Key: "↑↓", Desc: "select"}, {Key: "esc", Desc: m.escHint()}}
}

// state describes a session in a few words.
func state(s api.SessionInfo) string {
	switch s.Attached {
	case 0:
		return s.State
	case 1:
		return s.State + ", 1 client"
	default:
		return s.State + ", " + strconv.Itoa(s.Attached) + " clients"
	}
}

func indent(rows []string, x int) []string {
	pad := strings.Repeat(" ", x)
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = pad + r
	}
	return out
}

func center(s string, width int) string {
	return strings.Repeat(" ", max((width-ansi.StringWidth(s))/2, 0)) + s
}

// escHint says what Esc does: go back to the session the page was opened
// from, or quit.
func (m *Model) escHint() string {
	if m.back != "" {
		return "back to " + m.back
	}
	return "quit"
}
