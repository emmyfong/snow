package gitpane

import (
	"fmt"

	"charm.land/lipgloss/v2"
)

// commitBody shows only what the message will commit. File rows give way to
// the count and the message box as the pane gets shorter.
func (m *Model) commitBody(height int) []row {
	var staged []entry
	for _, e := range m.entries() {
		if e.staged {
			staged = append(staged, e)
		}
	}
	muted := lipgloss.NewStyle().Foreground(m.th.Muted)
	files := "files"
	if len(staged) == 1 {
		files = "file"
	}
	title := fmt.Sprintf("Commit %d staged %s", len(staged), files)
	if m.st.Omitted > 0 {
		title += " shown"
	}
	out := []row{{text: "  " + muted.Render(title)}}
	if m.st.Omitted > 0 {
		warning := row{text: "  " + muted.Render(fmt.Sprintf("%d more changes not shown", m.st.Omitted))}
		if height < 2 {
			return []row{warning}
		}
		out = append(out, warning)
	}
	room := max(height-len(out), 0)
	shown := min(len(staged), 5, room)
	if len(staged) > shown && room > 0 {
		shown = min(shown, room-1)
	}
	for _, e := range staged[:shown] {
		out = append(out, row{text: "    " + m.changeCell(e, max(m.width-4, 0))})
	}
	if len(staged) > shown && len(out) < height {
		out = append(out, row{text: "    " + muted.Render(fmt.Sprintf("and %d more", len(staged)-shown))})
	}
	return out
}
