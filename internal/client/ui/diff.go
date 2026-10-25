package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

// DiffOp is what a line of a diff does.
type DiffOp int

// Kinds of diff line.
const (
	DiffKeep DiffOp = iota
	DiffAdd
	DiffRemove
)

// DiffLine is one line of a diff, as plain text.
type DiffLine struct {
	Op   DiffOp
	Text string
}

// DiffOptions configures DiffView.
type DiffOptions struct {
	Width   int
	Lines   []DiffLine
	Context int // unchanged lines kept around each change
	MaxRows int // the most rows drawn; 0 means no limit
}

// DiffView draws a diff as rows at most Width cells wide: added lines
// start with "+" in Added, removed lines with "-" in Removed, and kept
// lines are muted. Kept lines further than Context from a change fold into
// one "..." row. Long lines are cut. Past MaxRows, a last row counts the
// lines left out.
func DiffView(th theme.Theme, o DiffOptions) string {
	if o.Width <= 0 || len(o.Lines) == 0 {
		return ""
	}
	near := make([]bool, len(o.Lines))
	for i, l := range o.Lines {
		if l.Op == DiffKeep {
			continue
		}
		for j := max(i-o.Context, 0); j <= min(i+o.Context, len(o.Lines)-1); j++ {
			near[j] = true
		}
	}
	muted := lipgloss.NewStyle().Foreground(th.Muted)
	styles := map[DiffOp]lipgloss.Style{
		DiffKeep: muted, DiffAdd: lipgloss.NewStyle().Foreground(th.Added), DiffRemove: lipgloss.NewStyle().Foreground(th.Removed),
	}
	signs := map[DiffOp]string{DiffKeep: " ", DiffAdd: "+", DiffRemove: "-"}
	var rows []string
	shown := 0 // lines drawn or folded so far
	folded := false
	for i, l := range o.Lines {
		if !near[i] {
			if !folded {
				rows = append(rows, muted.Render(ansi.Truncate("  ...", o.Width, "")))
				folded = true
			}
			shown++
			continue
		}
		folded = false
		if o.MaxRows > 0 && len(rows) == o.MaxRows-1 && i < len(o.Lines)-1 {
			rows = append(rows, muted.Render(ansi.Truncate(fmt.Sprintf("  ... %d more lines", len(o.Lines)-shown), o.Width, "")))
			break
		}
		rows = append(rows, styles[l.Op].Render(ansi.Truncate(signs[l.Op]+" "+PlainLine(l.Text), o.Width, "")))
		shown++
	}
	return strings.Join(rows, "\n")
}
