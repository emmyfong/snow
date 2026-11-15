package viewer

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/ui"
)

// View returns the pane's rows, each at most its width.
func (m *Model) View() []string {
	if m.width <= 0 || m.height <= 0 {
		return nil
	}
	rows := []string{m.header()}
	if note := m.noteLine(); note != "" && m.height >= 4 {
		rows = append(rows, note)
	}
	body := m.bodyRows()
	for _, r := range body {
		rows = append(rows, m.rowLine(r))
	}
	if text := m.emptyText(); text != "" && len(rows) < 1+m.bodyHeight() {
		rows = append(rows, " "+lipgloss.NewStyle().Foreground(m.th.Muted).Render(text))
	}
	if len(rows) < m.height-1 {
		rows = append(rows, make([]string, m.height-1-len(rows))...)
	}
	rows = append(rows, m.hints())
	rows = rows[:min(len(rows), m.height)]
	for i, r := range rows {
		rows[i] = ansi.Truncate(r, m.width, "")
	}
	return rows
}

// header is the first row: the path in bold and the turn that explains its
// marks, when one does.
func (m *Model) header() string {
	if m.st.Path == "" {
		return lipgloss.NewStyle().Bold(true).Render(" Viewer")
	}
	left := lipgloss.NewStyle().Bold(true).Render(" " + ui.PlainLine(m.st.Path))
	right := ""
	if m.st.Attribution != "" {
		right = lipgloss.NewStyle().Foreground(m.th.AgentBG).Render(ui.PlainLine(m.st.Attribution))
	}
	if right == "" {
		return left
	}
	gap := m.width - ansi.StringWidth(left) - ansi.StringWidth(right)
	if gap < 1 {
		return left
	}
	return left + strings.Repeat(" ", gap) + right
}

// noteLine is the row that names a limit, a stale mark set, or an error.
func (m *Model) noteLine() string {
	var parts []string
	if m.st.Stale {
		parts = append(parts, "Change evidence no longer matches this file.")
	}
	if m.st.Limit != "" {
		parts = append(parts, m.st.Limit)
	}
	if m.st.Error != "" {
		style := lipgloss.NewStyle().Foreground(m.th.ErrorFG).Background(m.th.Error)
		return style.Render(ansi.Truncate(" "+ui.PlainLine(m.st.Error), m.width, ""))
	}
	if len(parts) == 0 {
		return ""
	}
	text := strings.Join(parts, " ")
	return " " + lipgloss.NewStyle().Foreground(m.th.Muted).Render(ui.PlainLine(text))
}

// emptyText says why the body has no file lines.
func (m *Model) emptyText() string {
	switch {
	case !m.loaded:
		return "Reading the file..."
	case m.st.Binary:
		return "Binary file. Snow does not show it."
	case m.st.Error != "":
		return ""
	case m.st.Path == "":
		return "No file open. Use the files pane to open one."
	case len(m.st.Lines) == 0:
		return "This file is empty."
	}
	return ""
}

// rowLine draws one body row: its line number, change mark, and text.
func (m *Model) rowLine(r row) string {
	num := ""
	if r.line.Number > 0 {
		num = fmt.Sprintf("%d", r.line.Number)
	}
	width := m.numberWidth()
	num = strings.Repeat(" ", width-len(num)) + num
	gutter := lipgloss.NewStyle().Foreground(m.th.Muted).Render(num) + " "
	mark := " "
	text := r.text
	switch r.line.Mark {
	case "+":
		mark = lipgloss.NewStyle().Foreground(m.th.Added).Render("+")
	case "-":
		mark = lipgloss.NewStyle().Foreground(m.th.Removed).Render("-")
		text = lipgloss.NewStyle().Foreground(m.th.Removed).Render(ui.PlainLine(r.line.Text))
	}
	if text == "" && r.line.Mark != "-" {
		text = ui.PlainLine(r.line.Text)
	}
	return gutter + mark + " " + text
}

// numberWidth is the cells the line numbers take.
func (m *Model) numberWidth() int {
	last := 0
	for _, l := range m.st.Lines {
		if l.Number > last {
			last = l.Number
		}
	}
	w := len(fmt.Sprintf("%d", last))
	if w < 3 {
		return 3
	}
	return w
}

// hints are the keys the pane offers on its bottom row.
func (m *Model) hints() string {
	return " " + ui.KeyHints(m.th, []ui.Hint{{Key: "r", Desc: "refresh"}}, max(m.width-1, 0))
}
