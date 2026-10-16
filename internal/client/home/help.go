package home

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/ui"
)

// helpKeys are the keys the help panel lists after the commands.
var helpKeys = [][2]string{
	{"Enter", "open or create a session"},
	{"/", "type a command; Tab completes"},
	{"Ctrl+b s", "switch sessions; Esc goes back"},
	{"Ctrl+b d", "detach and come here"},
}

// keyWidth is the help panel's first column, wide enough for the longest
// command with its arguments.
const keyWidth = 26

// helpPanel draws the commands and keys in a frame centered on the screen.
func (m *Model) helpPanel() (box string, x, y int) {
	th := m.theme
	accent := lipgloss.NewStyle().Foreground(th.Accent)
	muted := lipgloss.NewStyle().Foreground(th.Muted)
	width := min(64, max(m.cols-4, 8))
	inner := max(width-4, 0)
	var rows []string
	add := func(key, desc string) {
		row := accent.Render(padRight(key, keyWidth)) + desc
		rows = append(rows, " "+ansi.Truncate(row, inner, "…"))
	}
	for _, c := range commands {
		add(strings.TrimSpace("/"+c.name+" "+c.args), c.desc)
	}
	rows = append(rows, "")
	for _, k := range helpKeys {
		add(k[0], k[1])
	}
	rows = append(rows, "", " "+muted.Render("Press any key to close."))
	box = ui.Panel(th, ui.PanelOptions{Title: "Help", Body: strings.Join(rows, "\n"), Width: width})
	w, h := ui.Size(box)
	return box, max((m.cols-w)/2, 0), max((m.rows-h)/2, 0)
}

func padRight(s string, n int) string {
	return s + strings.Repeat(" ", max(n-len([]rune(s)), 1))
}
