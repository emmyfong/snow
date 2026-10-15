// Package theme is the only source of Snow's colors, glyphs, and spacing.
// Views read tokens from a Theme; they never hold a literal color or glyph.
// The tokens here are placeholders until the home-page epic (#56) chooses
// the default theme.
package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Theme holds the visual tokens.
type Theme struct {
	Border      color.Color // lines between panes
	FocusBorder color.Color // lines around the focused pane
	BarFG       color.Color // status bar text
	BarBG       color.Color // status bar background
	TabActive   color.Color // the shown window's tab
	Muted       color.Color // secondary text
	Error       color.Color // a refused request's message
	PromptFG    color.Color // a question that takes the status bar
	PromptBG    color.Color
	Selection   color.Color // the selected item in a list
	IndicatorFG color.Color // copy mode's position marker
	IndicatorBG color.Color
	MatchFG     color.Color // a search match in copy mode
	MatchBG     color.Color
	CurrentFG   color.Color // the match the cursor is on
	CurrentBG   color.Color
	SelectedFG  color.Color // text selected for copying
	SelectedBG  color.Color
	Glyphs      Glyphs
}

// Glyphs are the characters that draw borders and markers.
type Glyphs struct {
	Vertical, Horizontal                       string
	Cross, TeeLeft, TeeRight, TeeUp, TeeDown   string
	Zoomed                                     string // marks a zoomed window's tab
	ActiveTab                                  string // marks the shown window's tab
	SessionLeft, SessionRight                  string // around the session name
	Pointer                                    string // marks the selected item in a list
	TopLeft, TopRight, BottomLeft, BottomRight string
}

// Default returns the placeholder theme.
func Default() Theme {
	return Theme{
		Border:      lipgloss.Color("8"),
		FocusBorder: lipgloss.Color("2"),
		BarFG:       lipgloss.Color("0"),
		BarBG:       lipgloss.Color("2"),
		TabActive:   lipgloss.Color("15"),
		Muted:       lipgloss.Color("8"),
		Error:       lipgloss.Color("1"),
		PromptFG:    lipgloss.Color("0"),
		PromptBG:    lipgloss.Color("3"),
		Selection:   lipgloss.Color("4"),
		IndicatorFG: lipgloss.Color("15"),
		IndicatorBG: lipgloss.Color("8"),
		MatchFG:     lipgloss.Color("0"),
		MatchBG:     lipgloss.Color("3"),
		CurrentFG:   lipgloss.Color("0"),
		CurrentBG:   lipgloss.Color("208"),
		SelectedFG:  lipgloss.Color("15"),
		SelectedBG:  lipgloss.Color("4"),
		Glyphs: Glyphs{
			Vertical: "│", Horizontal: "─",
			Cross: "┼", TeeLeft: "┤", TeeRight: "├", TeeUp: "┴", TeeDown: "┬",
			Zoomed: "Z", ActiveTab: "*",
			SessionLeft: "[", SessionRight: "]",
			Pointer: "> ",
			TopLeft: "┌", TopRight: "┐", BottomLeft: "└", BottomRight: "┘",
		},
	}
}
