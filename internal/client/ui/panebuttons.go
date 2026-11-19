package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/theme"
)

// PaneButton is a focused pane action and its cells in the window area.
type PaneButton struct {
	Action string
	X, Y   int
	W      int
}

// PaneButtons returns the focused pane's controls on its top border. Chat
// panes add a model picker before zoom and close. When the controls do not
// fit, only close remains. Rectangles describe pane content, so a pane whose
// Y is zero has no top border and cannot carry controls.
func PaneButtons(panes []WindowPane, width, height int) []PaneButton {
	if width <= 0 || height <= 0 {
		return nil
	}
	g := newGrid(WindowOptions{Width: width, Height: height, Panes: panes})
	return g.paneButtons(panes)
}

func (g *grid) paneButtons(panes []WindowPane) []PaneButton {
	if g.focused < 0 {
		return nil
	}
	p := panes[g.focused]
	segments := g.topSegments(p)
	if len(segments) == 0 {
		return nil
	}
	segment := segments[len(segments)-1]
	start, end := segment[0], segment[1]
	actions := []string{"zoom", "close-pane"}
	if p.Kind == "chat" {
		actions = append([]string{"model-picker"}, actions...)
	}
	// Leave one rule cell around the controls, and one between buttons.
	if end-start <= 12 || end-start < len(actions)*4+1 {
		actions = []string{"close-pane"}
	}
	inset := 0
	if end-start >= 5 {
		inset = 1
	}
	buttonWidth := min(3, end-start)
	x := end - inset - (buttonWidth+1)*len(actions) + 1
	buttons := make([]PaneButton, 0, len(actions))
	for _, action := range actions {
		buttons = append(buttons, PaneButton{Action: action, X: x, Y: p.Y - 1, W: buttonWidth})
		x += buttonWidth + 1
	}
	return buttons
}

// topSegments excludes every junction, including a divider ending above a
// pane that spans the next row. Labels and controls may replace rules only.
func (g *grid) topSegments(p WindowPane) [][2]int {
	y := p.Y - 1
	if y < 0 || y >= g.h || p.H <= 0 {
		return nil
	}
	var segments [][2]int
	start := -1
	for x := max(p.X, 0); x < min(p.X+p.W, g.w); x++ {
		straight := g.border(x, y) && !g.border(x, y-1) && !g.border(x, y+1)
		if straight && start < 0 {
			start = x
		}
		if !straight && start >= 0 {
			segments = append(segments, [2]int{start, x})
			start = -1
		}
	}
	if start >= 0 {
		segments = append(segments, [2]int{start, min(p.X+p.W, g.w)})
	}
	return segments
}

type paneDecoration struct {
	x, y, w int
	text    string
}

func (g *grid) paneDecorations(th theme.Theme, panes []WindowPane) []paneDecoration {
	buttons := g.paneButtons(panes)
	var decorations []paneDecoration
	for _, p := range panes {
		segments := g.topSegments(p)
		if len(segments) == 0 {
			continue
		}
		start, end := segments[0][0], segments[0][1]
		if end-start >= 3 {
			start++
			end--
		}
		for _, b := range buttons {
			if b.Y == p.Y-1 && b.X >= start && b.X < end {
				end = max(start, b.X-1)
			}
		}
		text := ansi.Truncate(PlainLine(p.Name), end-start, "")
		if ansi.StringWidth(text) > 0 {
			style := lipgloss.NewStyle().Foreground(th.Border)
			if p.Driven {
				style = style.Foreground(th.Accent)
			} else if p.Focused {
				style = style.Foreground(th.FocusBorder)
			}
			decorations = append(decorations, paneDecoration{x: start, y: p.Y - 1, w: ansi.StringWidth(text), text: style.Render(text)})
		}
	}
	for _, b := range buttons {
		glyph := th.Glyphs.Close
		style := lipgloss.NewStyle().Foreground(th.FocusBorder).Bold(true)
		switch b.Action {
		case "model-picker":
			glyph = th.Glyphs.Model
		case "zoom":
			glyph = th.Glyphs.Zoom
		case "close-pane":
			style = style.Foreground(th.Error)
		}
		text := ansi.Truncate(PlainLine(glyph), 1, "")
		text += strings.Repeat(" ", 1-ansi.StringWidth(text))
		switch b.W {
		case 3:
			text = th.Glyphs.SessionLeft + text + th.Glyphs.SessionRight
		case 2:
			text += " "
		}
		decorations = append(decorations, paneDecoration{x: b.X, y: b.Y, w: b.W, text: style.Render(text)})
	}
	return decorations
}
