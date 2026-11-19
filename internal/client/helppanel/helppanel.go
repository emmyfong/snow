// Package helppanel draws the help panel of the home page and of a chat:
// the commands the registry holds for that screen, then its keys, in a frame
// centered on the screen.
package helppanel

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/command"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
)

// Key is one key row: the key, and what it does.
type Key struct{ Keys, Desc string }

// keyWidth is the first column, wide enough for the longest command with
// its arguments.
const keyWidth = 26

// closeLine tells the user how to leave the panel.
const closeLine = "Press any key to close."

// Panel draws the commands cmds, then keys, in a frame centered on a screen
// of cols by rows cells. A screen too short for every row shows a marker for
// the rows it hides before the close line.
func Panel(th theme.Theme, cols, rows int, cmds []command.Command, keys []Key) (box string, x, y int) {
	accent := lipgloss.NewStyle().Foreground(th.Accent)
	muted := lipgloss.NewStyle().Foreground(th.Muted)
	width := min(64, max(cols-4, 8))
	inner := max(width-4, 0)
	var body []string
	add := func(key, desc string) {
		row := accent.Render(padRight(key, keyWidth)) + desc
		body = append(body, " "+ansi.Truncate(row, inner, "…"))
	}
	for _, c := range cmds {
		add("/"+c.Line(), c.Desc)
	}
	body = append(body, "")
	for _, k := range keys {
		add(k.Keys, k.Desc)
	}
	// Two rows are the frame, two more the blank row and the close line.
	if room := max(rows-4, 0); len(body) > room {
		if room == 0 {
			body = nil
		} else {
			kept := body[:room-1]
			if len(kept) > 0 && kept[len(kept)-1] == "" {
				kept = kept[:len(kept)-1]
			}
			hidden := len(body) - len(kept)
			marker := fmt.Sprintf("... %d more rows; make the window taller to see them", hidden)
			body = append(kept, muted.Render(" "+ansi.Truncate(marker, max(inner-1, 0), "…")))
		}
	}
	body = append(body, "", " "+muted.Render(closeLine))
	box = ui.Panel(th, ui.PanelOptions{Title: "Help", Body: strings.Join(body, "\n"), Width: width})
	w, h := ui.Size(box)
	return box, max((cols-w)/2, 0), max((rows-h)/2, 0)
}

func padRight(s string, n int) string {
	return s + strings.Repeat(" ", max(n-len([]rune(s)), 1))
}
