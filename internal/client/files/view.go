package files

import (
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
	for _, r := range m.bodyRows() {
		rows = append(rows, m.rowLine(r))
	}
	if need := 1 + m.bodyHeight() - len(rows); need > 0 {
		rows = append(rows, m.messageRows(need)...)
	}
	if m.st.Error != "" && m.height >= 4 {
		rows = append(rows, m.errorRow())
	}
	rows = append(rows, m.hints())
	if len(rows) < m.height {
		rows = append(rows, make([]string, m.height-len(rows))...)
	}
	rows = rows[:m.height]
	for i, r := range rows {
		rows[i] = ansi.Truncate(r, m.width, "")
	}
	return rows
}

// header is the first row: the folder in bold, and "reading" while the
// server reads it.
func (m *Model) header() string {
	bold := lipgloss.NewStyle().Bold(true)
	muted := lipgloss.NewStyle().Foreground(m.th.Muted)
	name := ui.FolderName(m.st.Folder)
	if m.st.Folder == "" {
		name = "Files"
	}
	left := bold.Render(" " + ui.PlainLine(name))
	if m.st.Loading {
		right := muted.Render("reading ")
		if gap := m.width - ansi.StringWidth(left) - ansi.StringWidth(right); gap >= 1 {
			return left + strings.Repeat(" ", gap) + right
		}
	}
	return left
}

// bodyRows returns the tree rows the body shows.
func (m *Model) bodyRows() []row {
	rows := m.rows()
	if len(rows) == 0 {
		return nil
	}
	h := m.bodyHeight()
	first := min(max(m.top, 0), max(len(rows)-h, 0))
	last := min(first+h, len(rows))
	return rows[first:last]
}

// rowLine draws one tree row.
func (m *Model) rowLine(r row) string {
	prefix := strings.Repeat("  ", r.depth)
	if r.entry.Dir {
		if m.expanded(r.entry.Path) {
			prefix += m.th.Glyphs.Expanded + " "
		} else {
			prefix += m.th.Glyphs.Collapsed + " "
		}
	} else {
		prefix += "  "
	}
	left := ui.PlainLine(prefix + r.entry.Name)
	mark := r.entry.Mark
	markCell := 0
	if mark != "" {
		markCell = 1
	}
	left = ansi.Truncate(left, max(m.width-markCell, 1), "")
	pad := max(m.width-ansi.StringWidth(left)-markCell, 0)
	if r.entry.Path == m.sel {
		style := lipgloss.NewStyle().Foreground(m.th.SelectionFG).Background(m.th.Selection).Bold(true)
		return style.Render(left + strings.Repeat(" ", pad) + mark)
	}
	if mark == "" {
		return left + strings.Repeat(" ", pad)
	}
	return left + strings.Repeat(" ", pad) + m.markStyle(mark).Render(mark)
}

// markStyle colors git's one-letter mark.
func (m *Model) markStyle(mark string) lipgloss.Style {
	switch mark {
	case "A", "?":
		return lipgloss.NewStyle().Foreground(m.th.Added)
	case "D", "R", "U":
		return lipgloss.NewStyle().Foreground(m.th.Removed)
	}
	return lipgloss.NewStyle().Foreground(m.th.Accent)
}

// messageRows fills empty body rows with what the pane has to say.
func (m *Model) messageRows(n int) []string {
	if n <= 0 {
		return nil
	}
	muted := lipgloss.NewStyle().Foreground(m.th.Muted)
	var text string
	switch {
	case !m.loaded:
		text = "Reading the folder..."
	case len(m.rows()) == 0 && m.st.Error == "":
		text = "This folder has no files."
	}
	if text == "" {
		return make([]string, n)
	}
	out := []string{" " + muted.Render(text)}
	for len(out) < n {
		out = append(out, "")
	}
	return out
}

// errorRow is the last failure, in the error colors.
func (m *Model) errorRow() string {
	style := lipgloss.NewStyle().Foreground(m.th.ErrorFG).Background(m.th.Error)
	return style.Render(ansi.Truncate(" "+ui.PlainLine(m.st.Error), m.width, ""))
}

// hints are the keys the pane offers on its bottom row.
func (m *Model) hints() string {
	return " " + ui.KeyHints(m.th, []ui.Hint{
		{Key: "Enter", Desc: "view"},
	}, max(m.width-1, 0))
}

// Cursor says where the terminal's cursor goes; the files pane has none.
func (m *Model) Cursor() (x, y int, ok bool) { return 0, 0, false }
