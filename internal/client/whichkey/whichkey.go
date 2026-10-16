// Package whichkey is the prefix hint popup: after the prefix key and a short
// pause it lists the keys that can follow, with what each does. Experts who
// type the second key at once never see it.
package whichkey

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/keys"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
)

// Options configures View.
type Options struct {
	Prefix    string       // the prefix key written for people, for the title
	Entries   []keys.Entry // from keys.Table.Entries
	MaxWidth  int          // the popup never draws wider than this
	MaxHeight int          // nor taller; a long list moves to two columns
}

// gap separates a key from its description, and one column from the next.
const gap = "  "

// View draws the popup, or "" when there is nothing to show or no room. One
// row per entry; when that is too tall, the entries flow into two columns.
// A popup too small for its rows cuts them.
func View(th theme.Theme, o Options) string {
	if len(o.Entries) == 0 || o.MaxWidth < 4 || o.MaxHeight < 3 {
		return ""
	}
	room := o.MaxHeight - 2
	cols := 1
	if len(o.Entries) > room {
		cols = 2
	}
	per := (len(o.Entries) + cols - 1) / cols
	var columns [][]string
	for c := range cols {
		part := o.Entries[min(c*per, len(o.Entries)):min((c+1)*per, len(o.Entries))]
		columns = append(columns, column(part))
	}
	items := make([]string, min(per, room))
	for i := range items {
		var parts []string
		for c, col := range columns {
			cell := ""
			if i < len(col) {
				cell = col[i]
			}
			if c < len(columns)-1 {
				cell += strings.Repeat(" ", columnWidth(col)-ansi.StringWidth(cell))
			}
			parts = append(parts, cell)
		}
		items[i] = strings.Join(parts, gap+gap)
	}
	// The frame takes two cells, and one more keeps text off the right edge.
	width := ansi.StringWidth(th.Glyphs.Pointer) + 3
	for _, it := range items {
		width = max(width, ansi.StringWidth(th.Glyphs.Pointer)+ansi.StringWidth(it)+3)
	}
	title := o.Prefix
	return ui.Popup(th, ui.PopupOptions{Title: title, Items: items, Selected: -1, Width: min(width, o.MaxWidth)})
}

// column renders entries as aligned "keys  description" cells.
func column(es []keys.Entry) []string {
	keyW := 0
	for _, e := range es {
		keyW = max(keyW, ansi.StringWidth(e.Keys))
	}
	bold := lipgloss.NewStyle().Bold(true)
	out := make([]string, len(es))
	for i, e := range es {
		pad := strings.Repeat(" ", keyW-ansi.StringWidth(e.Keys))
		out[i] = bold.Render(e.Keys) + pad + gap + e.Desc
	}
	return out
}

func columnWidth(col []string) int {
	w := 0
	for _, c := range col {
		w = max(w, ansi.StringWidth(c))
	}
	return w
}
