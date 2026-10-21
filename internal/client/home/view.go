package home

import (
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// placement is where the page's parts go for the current size, shared by
// drawing and by clicks.
type placement struct {
	header         bool // wordmark and version fit
	top            int  // first row of the page
	x, width       int  // the column and width of the input, list, and hints
	listTop, lines int  // the first row of the main list and its rows
	first          int  // index of the first item shown
	count          int  // items shown
	sessions       band // the sessions under the command list; empty otherwise
}

// band is a second list on the page: the sessions while a command is typed.
type band struct {
	top, lines   int // the first row of the rows and how many there are
	first, count int // index of the first session shown, and sessions shown
}

// Rows of each part. The header is the wordmark, the version, and a blank.
const (
	headerRows = ui.WordmarkHeight + 2
	inputRows  = 3                         // a framed one-line input
	fixedRows  = inputRows + 1 + 1 + 1 + 1 // input, blank, label, blank, hints
	bandRows   = 2                         // a blank and the label above a band
)

// layout places the page in the screen: centered, the list as tall as the
// room allows, the wordmark dropped first when the screen is short. While a
// command is typed, the sessions follow the commands and shrink first.
func (m *Model) layout() placement {
	p := placement{width: min(64, max(m.cols-4, 0))}
	p.x = max((m.cols-p.width)/2, 0)
	shown, selected := m.listLen(), m.listSel()
	notes := len(m.noteLines())
	room := m.rows - fixedRows - notes
	p.header = m.cols >= ui.WordmarkWidth && room-headerRows >= min(max(shown, 1), 3)
	if p.header {
		room -= headerRows
	}
	p.lines = min(max(shown, 1), max(room, 1))
	total := fixedRows + p.lines + notes
	if p.header {
		total += headerRows
	}
	if m.commanding() && !m.picking {
		if n := min(max(len(m.sessions), 1), room-p.lines-bandRows); n > 0 {
			p.sessions.lines = n
			total += bandRows + n
		}
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
	if b := &p.sessions; b.lines > 0 {
		b.top = p.listTop + p.lines + bandRows
		b.count = min(len(m.sessions), b.lines)
		if t := m.targetIndex(); t >= b.count {
			b.first = t - b.count + 1
		}
	}
	return p
}

// targetIndex is the index in the sessions of the session a command acts
// on, or -1.
func (m *Model) targetIndex() int {
	return slices.IndexFunc(m.sessions, func(s api.SessionInfo) bool { return s.Name == m.target })
}

// listLen and listSel are the list's length and selected row: the matching
// commands while a slash command is typed, else the sessions.
func (m *Model) listLen() int {
	if m.picking {
		return len(theme.Builtin())
	}
	if m.commanding() {
		return len(m.matches())
	}
	return len(m.shown())
}

func (m *Model) listSel() int {
	if m.picking {
		return m.themeSel
	}
	if m.commanding() {
		return m.cmdSel
	}
	if m.naming != nil {
		return slices.IndexFunc(m.sessions, func(s api.SessionInfo) bool { return s.Name == m.naming.session })
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

// rowAt returns the index of the session on screen row y.
func (b band) rowAt(y int) (int, bool) {
	if y < b.top || y >= b.top+b.count {
		return 0, false
	}
	return b.first + y - b.top, true
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
	in := ui.InputOptions{
		Value: m.input, Placeholder: "session name, Enter for a new one, / for commands",
		Prompt: th.Glyphs.Pointer, Width: max(p.width-2, 0), Focused: true,
	}
	if m.naming != nil {
		in.Value, in.Placeholder, in.Prompt = m.naming.name, "", m.namingPrompt()
	}
	field := ui.Input(th, in)
	lines = append(lines, indent(strings.Split(ui.Panel(th, ui.PanelOptions{Body: field, Width: p.width}), "\n"), p.x)...)
	notice := ""
	if m.notice != "" {
		notice = indent([]string{lipgloss.NewStyle().Foreground(th.Error).Render(ansi.Truncate(m.notice, p.width, "…"))}, p.x)[0]
	}
	lines = append(lines, notice)
	switch {
	case m.picking:
		lines = append(lines, m.themeList(p)...)
	case m.commanding():
		lines = append(lines, m.commandList(p)...)
		if b := p.sessions; b.lines > 0 {
			lines = append(lines, "")
			lines = append(lines, m.sessionList(p.x, p.width, b, m.sessions, m.targetIndex())...)
		}
	default:
		lines = append(lines, m.sessionList(p.x, p.width, band{first: p.first, count: p.count}, m.shown(), m.listSel())...)
		lines = append(lines, indent(m.noteLines(), p.x)...)
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

// sessionList draws the label and the sessions b shows, from x, width
// wide, with sessions[selected] highlighted.
func (m *Model) sessionList(x, width int, b band, sessions []api.SessionInfo, selected int) []string {
	th := m.theme
	muted := lipgloss.NewStyle().Foreground(th.Muted)
	lines := []string{indent([]string{muted.Render("Sessions")}, x)[0]}
	if b.count == 0 {
		return append(lines, indent([]string{muted.Render("No sessions yet. Type a name and press Enter.")}, x)[0])
	}
	shown := sessions[b.first : b.first+b.count]
	items := make([]ui.ListItem, len(shown))
	for i, s := range shown {
		items[i] = ui.ListItem{Cols: []string{s.Name, state(s), ago(m.now().Sub(s.LastUsed))}}
	}
	list := ui.List(th, ui.ListOptions{Items: items, Selected: selected - b.first, Width: width})
	return append(lines, indent(strings.Split(list, "\n"), x)...)
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
	if m.picking {
		return []ui.Hint{{Key: "enter", Desc: "keep"}, {Key: "↑↓", Desc: "preview"}, {Key: "esc", Desc: "cancel"}}
	}
	if m.naming != nil {
		return []ui.Hint{{Key: "enter", Desc: "rename"}, {Key: "esc", Desc: "back"}}
	}
	if m.pickingSession() {
		return []ui.Hint{{Key: "enter", Desc: "run"}, {Key: "↑↓", Desc: "pick session"}, {Key: "esc", Desc: "clear"}}
	}
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

// maxNotes bounds the note lines on the page; the rest are counted.
const maxNotes = 3

// noteLines are the server's notes on saved sessions, shown under the
// sessions: files it could not read, or saving that fails. A long path keeps its end, where the file name
// is.
func (m *Model) noteLines() []string {
	if m.picking || m.commanding() || len(m.notes) == 0 {
		return nil
	}
	style := lipgloss.NewStyle().Foreground(m.theme.Error)
	width := min(64, max(m.cols-4, 0))
	var out []string
	for i, note := range m.notes {
		if i == maxNotes-1 && len(m.notes) > maxNotes {
			note = strconv.Itoa(len(m.notes)-i) + " more notes about saved sessions; see the log"
		} else if i >= maxNotes {
			break
		}
		if ansi.StringWidth(note) > width {
			note = "…" + ansi.TruncateLeft(note, ansi.StringWidth(note)-width+1, "")
		}
		out = append(out, style.Render(note))
		if len(out) == maxNotes {
			break
		}
	}
	return out
}
