package chat

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// maxTaskRows bounds the rows of the open task list, below the third of
// the pane it may take.
const maxTaskRows = 12

// taskRows are the agent's task list, drawn above the input: one row that
// counts what is done and names the task in progress, and, while the list
// is open, a row per task. No list draws nothing.
func (m *Model) taskRows() []string {
	tasks := m.snap.Tasks
	if len(tasks) == 0 || m.width <= 0 {
		return nil
	}
	g := m.th.Glyphs
	muted := lipgloss.NewStyle().Foreground(m.th.Muted)
	accent := lipgloss.NewStyle().Foreground(m.th.Accent)
	done, current := 0, -1
	for i, t := range tasks {
		switch {
		case t.State == api.ChatTaskDone:
			done++
		case t.State == api.ChatTaskInProgress && current < 0:
			current = i
		}
	}
	mark, hint := g.Collapsed, "Ctrl+T show"
	if m.tasksOpen {
		mark, hint = g.Expanded, "Ctrl+T hide"
	}
	head := " " + mark + " Tasks " + accent.Render(fmt.Sprintf("%d/%d done", done, len(tasks)))
	if current >= 0 && !m.tasksOpen {
		head += "  " + g.Working + " " + ui.PlainLine(tasks[current].Text)
	}
	rows := []string{withHint(head, muted.Render(hint), m.width)}
	if !m.tasksOpen {
		return rows
	}
	room := min(maxTaskRows, max(m.height/3, 2)) - 1
	shown := tasks
	if len(tasks) > room {
		shown = tasks[:max(room-1, 0)]
	}
	for _, t := range shown {
		var row string
		switch t.State {
		case api.ChatTaskDone:
			row = muted.Render(g.Checked + " " + ui.PlainLine(t.Text))
		case api.ChatTaskInProgress:
			row = accent.Bold(true).Render(g.Working + " " + ui.PlainLine(t.Text))
		default:
			row = g.Unchecked + " " + ui.PlainLine(t.Text)
		}
		rows = append(rows, ansi.Truncate("   "+row, m.width, "…"))
	}
	if n := len(tasks) - len(shown); n > 0 {
		rows = append(rows, muted.Render(ansi.Truncate(fmt.Sprintf("   ... %d more", n), m.width, "")))
	}
	return rows
}

// withHint puts hint at the right end of row, width cells wide; the hint
// goes first when both do not fit.
func withHint(row, hint string, width int) string {
	gap := width - ansi.StringWidth(row) - ansi.StringWidth(hint)
	if gap < 1 {
		return ansi.Truncate(row, width, "…")
	}
	return row + strings.Repeat(" ", gap) + hint
}

// toggleTasks opens or closes the task list.
func (m *Model) toggleTasks() {
	if len(m.snap.Tasks) > 0 {
		m.tasksOpen = !m.tasksOpen
	}
}
