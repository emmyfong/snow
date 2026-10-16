package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

// PopupOptions configures Popup.
type PopupOptions struct {
	Title    string
	Items    []string
	Hints    []string // optional: Hints[i] sits at the right end of item i, muted, for example a shortcut
	Selected int      // index of the highlighted item; out of range means none
	Width    int      // total width, frame included
}

// Popup draws a framed list exactly Width cells wide: the title in the top
// edge and one item per row, the selected one marked and highlighted. Items
// that do not fit are cut. Items may hold styles. A hint is right-aligned
// and dropped, not cut, when it would touch its item. It draws nothing
// narrower than 4 cells.
func Popup(th theme.Theme, o PopupOptions) string {
	if o.Width < 4 {
		return ""
	}
	g := th.Glyphs
	selected := lipgloss.NewStyle().Background(th.Selection).Bold(true)
	muted := lipgloss.NewStyle().Foreground(th.Muted)
	inner := o.Width - 2
	rows := make([]string, len(o.Items))
	for i, item := range o.Items {
		marker := strings.Repeat(" ", ansi.StringWidth(g.Pointer))
		if i == o.Selected {
			marker = g.Pointer
		}
		text := ansi.Truncate(marker+item, inner, "")
		hint := ""
		if i < len(o.Hints) {
			hint = o.Hints[i]
		}
		// Two cells between an item and its hint, and one before the frame.
		room := inner - ansi.StringWidth(marker+item) - ansi.StringWidth(hint) - 1
		if hint != "" && room >= 2 {
			styled := muted.Render(hint)
			if i == o.Selected {
				styled = hint
			}
			text += strings.Repeat(" ", room) + styled + " "
		} else {
			text += strings.Repeat(" ", inner-ansi.StringWidth(text))
		}
		if i == o.Selected {
			text = selected.Render(text)
		}
		rows[i] = text
	}
	return frame(th, o.Title, rows, o.Width)
}
