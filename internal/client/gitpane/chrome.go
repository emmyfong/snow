package gitpane

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/ui"
)

// header is the first row: the branch in bold, then the folder and how far
// the branch is from its upstream, muted; "reading" while git runs.
func (m *Model) header() string {
	bold := lipgloss.NewStyle().Bold(true)
	muted := lipgloss.NewStyle().Foreground(m.th.Muted)
	var left string
	switch {
	case m.st.Detached:
		left = bold.Render(" detached HEAD")
	case m.st.Branch != "":
		left = bold.Render(" " + ui.PlainLine(m.st.Branch))
	default:
		left = bold.Render(" Git")
	}
	var extra []string
	if m.loaded && m.st.Folder != "" {
		extra = append(extra, "in "+ui.FolderShort(m.st.Folder))
	}
	switch {
	case m.st.Initial:
		extra = append(extra, "no commits yet")
	case m.st.Ahead > 0 && m.st.Behind > 0:
		extra = append(extra, fmt.Sprintf("%d ahead, %d behind", m.st.Ahead, m.st.Behind))
	case m.st.Ahead > 0:
		extra = append(extra, fmt.Sprintf("%d ahead", m.st.Ahead))
	case m.st.Behind > 0:
		extra = append(extra, fmt.Sprintf("%d behind", m.st.Behind))
	}
	// Each part shows whole or not at all, the folder first.
	for i, e := range extra {
		sep := ", "
		if i == 0 {
			sep = "  "
		}
		if ansi.StringWidth(left)+ansi.StringWidth(sep+e) > m.width {
			break
		}
		left += muted.Render(sep + e)
	}
	if m.st.Loading {
		right := muted.Render("reading ")
		if gap := m.width - ansi.StringWidth(left) - ansi.StringWidth(right); gap >= 1 {
			return left + strings.Repeat(" ", gap) + right
		}
	}
	return left
}

// statusLines are the unstyled rows of the last outcome and whether they
// omit text. Errors wrap to three rows; notices stay on one row.
func (m *Model) statusLines() ([]string, bool) {
	if m.removePath != "" {
		return []string{" Remove worktree " + ui.FolderName(m.removePath) + "? y removes it, n keeps it."}, false
	}
	if m.st.Error == "" {
		text := m.st.Notice
		if m.note != "" {
			text = m.note
		}
		return []string{" " + ui.PlainLine(text)}, false
	}
	const maxErrorRows = 3
	lines := strings.Split(ansi.Wrap(" "+ui.PlainLine(m.st.Error), max(m.width, 1), ""), "\n")
	omitted := len(lines) > maxErrorRows
	if omitted {
		lines = lines[:maxErrorRows]
	}
	return lines, omitted
}

// statusRows render at most limit visible rows and mark an error when text
// remains beyond the last row that fits.
func (m *Model) statusRows(limit int) []string {
	lines, omitted := m.statusLines()
	if limit < len(lines) {
		lines = lines[:max(limit, 0)]
		omitted = true
	}
	if len(lines) == 0 {
		return nil
	}
	if m.removePath != "" {
		text := ansi.Truncate(lines[0], m.width, "…")
		return []string{lipgloss.NewStyle().Foreground(m.th.PromptFG).Background(m.th.PromptBG).Render(text)}
	}
	if m.st.Error == "" {
		if m.note != "" {
			return []string{lipgloss.NewStyle().Foreground(m.th.Muted).Render(ansi.Truncate(lines[0], m.width, "…"))}
		}
		return []string{lipgloss.NewStyle().Foreground(m.th.Success).Render(ansi.Truncate(lines[0], m.width, ""))}
	}
	if omitted && m.width > 0 {
		last := lines[len(lines)-1]
		if m.width == 1 {
			last = "…"
		} else {
			last = ansi.Truncate(last, m.width-1, "") + "…"
		}
		lines[len(lines)-1] = last
	}
	style := lipgloss.NewStyle().Foreground(m.th.ErrorFG).Background(m.th.Error)
	out := make([]string, len(lines))
	for i, line := range lines {
		line = ansi.Truncate(line, m.width, "")
		out[i] = style.Render(line + strings.Repeat(" ", max(m.width-ansi.StringWidth(line), 0)))
	}
	return out
}

func (m *Model) boxOptions() ui.TextAreaOptions {
	return ui.TextAreaOptions{
		Width: max(m.width-1, 1), Height: boxRows, Prompt: boxPrompt,
		Value: string(m.msg), Cursor: m.cursor,
		Placeholder: "Enter commits. Esc closes and keeps the draft.",
	}
}

func (m *Model) listHints() []hint {
	switch {
	case m.removePath != "":
		return []hint{
			{ui.Hint{Key: "y", Desc: "remove"}, tea.Key{Code: 'y', Text: "y"}},
			{ui.Hint{Key: "n", Desc: "keep"}, tea.Key{Code: 'n', Text: "n"}},
		}
	}
	return nil
}
