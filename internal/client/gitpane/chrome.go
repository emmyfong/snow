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
		extra = append(extra, "in "+ui.FolderName(m.st.Folder))
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

// statusRow is the last action's failure, in the error colors, or its
// outcome.
func (m *Model) statusRow() string {
	if m.st.Error != "" {
		style := lipgloss.NewStyle().Foreground(m.th.ErrorFG).Background(m.th.Error)
		text := ansi.Truncate(" "+ui.PlainLine(m.st.Error), m.width, "...")
		return style.Render(text + strings.Repeat(" ", max(m.width-ansi.StringWidth(text), 0)))
	}
	return lipgloss.NewStyle().Foreground(m.th.Success).Render(" " + ui.PlainLine(m.st.Notice))
}

func (m *Model) boxOptions() ui.TextAreaOptions {
	return ui.TextAreaOptions{
		Width: max(m.width-1, 1), Height: boxRows, Prompt: boxPrompt,
		Value: string(m.msg), Cursor: m.cursor,
		Placeholder: "Enter commits, Esc cancels",
	}
}

func (m *Model) listHints() []hint {
	refresh := hint{ui.Hint{Key: "r", Desc: "refresh"}, tea.Key{Code: 'r', Text: "r"}}
	switch {
	case m.mode == modeCommit:
		return []hint{
			{ui.Hint{Key: "enter", Desc: "commit"}, tea.Key{Code: tea.KeyEnter}},
			{ui.Hint{Key: "ctrl+j", Desc: "new line"}, tea.Key{Code: 'j', Mod: tea.ModCtrl}},
			{ui.Hint{Key: "esc", Desc: "cancel"}, tea.Key{Code: tea.KeyEscape}},
		}
	case len(m.entries()) == 0:
		return []hint{refresh}
	}
	hs := []hint{
		{ui.Hint{Key: "s", Desc: "stage"}, tea.Key{Code: 's', Text: "s"}},
		{ui.Hint{Key: "u", Desc: "unstage"}, tea.Key{Code: 'u', Text: "u"}},
		{ui.Hint{Key: "c", Desc: "commit"}, tea.Key{Code: 'c', Text: "c"}},
	}
	es := m.entries()
	if i := m.selected(es); i < 0 || es[i].cm == nil {
		enter := "diff"
		if i >= 0 && es[i].wt != nil {
			enter = "open"
		}
		hs = append(hs, hint{ui.Hint{Key: "enter", Desc: enter}, tea.Key{Code: tea.KeyEnter}})
	}
	return append(hs,
		hint{ui.Hint{Key: "a", Desc: "stage all"}, tea.Key{Code: 'a', Text: "a"}},
		refresh,
	)
}
