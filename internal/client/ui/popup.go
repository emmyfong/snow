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
	frame := lipgloss.NewStyle().Foreground(th.FocusBorder)
	selected := lipgloss.NewStyle().Background(th.Selection).Bold(true)
	muted := lipgloss.NewStyle().Foreground(th.Muted)
	inner := o.Width - 2

	title := ansi.Truncate(o.Title, max(inner-2, 0), "")
	top := g.TopLeft + g.Horizontal
	if title != "" {
		top += " " + title + " "
	}
	top = ansi.Truncate(top, o.Width-1, "")
	top += strings.Repeat(g.Horizontal, max(o.Width-1-ansi.StringWidth(top), 0)) + g.TopRight
	rows := []string{frame.Render(top)}
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
		rows = append(rows, frame.Render(g.Vertical)+text+frame.Render(g.Vertical))
	}
	rows = append(rows, frame.Render(g.BottomLeft+strings.Repeat(g.Horizontal, inner)+g.BottomRight))
	return strings.Join(rows, "\n")
}
