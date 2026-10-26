package home

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/command"
	"github.com/emmyfong/snow/internal/client/sessionlist"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

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
		lines = append(lines, m.sessionList(p.x, p.width, p.list, m.shown(), m.listSel())...)
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
	list := ui.List(th, ui.ListOptions{Items: sessionlist.Items(shown, m.now()), Selected: selected - b.first, Width: width})
	return append(lines, indent(strings.Split(list, "\n"), x)...)
}

// commandList draws the label and the matching commands that fit.
func (m *Model) commandList(p placement) []string {
	th := m.theme
	muted := lipgloss.NewStyle().Foreground(th.Muted)
	lines := []string{indent([]string{muted.Render("Commands")}, p.x)[0]}
	ms := m.matches()
	if p.list.count == 0 {
		return append(lines, indent([]string{muted.Render("No command matches. Type /help to see them all.")}, p.x)[0])
	}
	list := ui.List(th, ui.ListOptions{
		Items:    command.NewList(ms[p.list.first:p.list.first+p.list.count], false, true).Items,
		Selected: m.cmdSel - p.list.first, Width: p.width,
	})
	return append(lines, indent(strings.Split(list, "\n"), p.x)...)
}

// hints are the keys the page answers to now.
func (m *Model) hints() []ui.Hint {
	if m.picking {
		return []ui.Hint{{Key: "enter", Desc: "keep"}, {Key: "↑↓", Desc: "preview"}, {Key: "esc", Desc: "cancel"}}
	}
	if m.naming != nil {
		return []ui.Hint{{Key: "enter", Desc: "rename"}, {Key: "ctrl+u", Desc: "clear"}, {Key: "esc", Desc: "back"}}
	}
	if m.pickingSession() {
		return []ui.Hint{{Key: "enter", Desc: "run"}, {Key: "↑↓", Desc: "pick session"}, {Key: "esc", Desc: "clear"}}
	}
	if m.commanding() {
		return []ui.Hint{{Key: "enter", Desc: "run"}, {Key: "tab", Desc: "complete"}, {Key: "↑↓", Desc: "select"}, {Key: "esc", Desc: "clear"}}
	}
	return []ui.Hint{{Key: "enter", Desc: "open or create"}, {Key: "/", Desc: "commands"}, {Key: "↑↓", Desc: "select"}, {Key: "esc", Desc: "quit"}}
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
