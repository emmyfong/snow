package theme

import (
	"image/color"
	"math"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

// minContrast is the WCAG contrast ratio for bold interface text. Text on a
// colored background below it is hard to read.
const minContrast = 3.0

var (
	black = lipgloss.Color("#000000")
	white = lipgloss.Color("#ffffff")
)

// For returns t for a terminal that shows profile p's colors. Reducing
// colors to 256 or 16 can make text vanish into its background; such text
// turns black or white, whichever reads better. A true-color terminal gets
// t unchanged. A terminal that shows no colors may lack the round agent
// marks too, so it gets ASCII ones.
func (t Theme) For(p colorprofile.Profile) Theme {
	if p < colorprofile.ANSI {
		t.Glyphs.AgentIdle, t.Glyphs.AgentWorking, t.Glyphs.AgentNeedsYou = "-", "~", "!"
		t.Glyphs.Driven = "*"
		return t // no colors to adapt
	}
	fix := func(fg *color.Color, bg color.Color) {
		if contrast(p.Convert(*fg), p.Convert(bg)) < minContrast {
			*fg = ink(p, bg)
		}
	}
	fix(&t.BarFG, t.BarBG)
	fix(&t.TabActive, t.TabActiveBG)
	fix(&t.PromptFG, t.PromptBG)
	fix(&t.ErrorFG, t.Error)
	fix(&t.IndicatorFG, t.IndicatorBG)
	fix(&t.MatchFG, t.MatchBG)
	fix(&t.CurrentFG, t.CurrentBG)
	fix(&t.SelectedFG, t.SelectedBG)
	fix(&t.SelectionFG, t.Selection)
	fix(&t.UserFG, t.UserBG)
	fix(&t.AgentFG, t.AgentBG)
	fix(&t.ActivityFG, t.ActivityBG)
	return t
}

// ink returns black or white, whichever reads better on bg in profile p.
func ink(p colorprofile.Profile, bg color.Color) color.Color {
	b := p.Convert(bg)
	if contrast(p.Convert(black), b) >= contrast(p.Convert(white), b) {
		return black
	}
	return white
}

// contrast is the WCAG contrast ratio of two colors, from 1 to 21.
func contrast(a, b color.Color) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func luminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	lin := func(v uint32) float64 {
		s := float64(v) / 0xffff
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}
