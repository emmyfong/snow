// Package theme is the only source of Snow's colors, glyphs, and spacing.
// Views read tokens from a resolved Theme; they never hold a literal color or
// glyph. Builtin lists the themes that ship with Snow; Frost is the default.
package theme

import (
	"errors"
	"fmt"
	"image/color"
	"strings"
)

// Theme holds the visual tokens. Snow keeps the terminal's own background
// and text colors; tokens color only Snow's parts.
type Theme struct {
	Name        string
	Border      color.Color // lines between panes
	FocusBorder color.Color // lines around the focused pane
	BarFG       color.Color // status bar text
	BarBG       color.Color // status bar background
	TabActive   color.Color // the shown window's tab text
	TabActiveBG color.Color // the shown window's tab background
	Accent      color.Color // highlights: the status bar in built-in themes, the prompt mark, commands, links
	Muted       color.Color // secondary text
	Success     color.Color // something that worked or runs
	Error       color.Color // a refused request's message: its background
	ErrorFG     color.Color // a refused request's message: its text
	PromptFG    color.Color // a question that takes the status bar
	PromptBG    color.Color
	Selection   color.Color // the selected item in a list
	SelectionFG color.Color // its text, black or white for contrast
	IndicatorFG color.Color // copy mode's position marker
	IndicatorBG color.Color
	MatchFG     color.Color // a search match in copy mode
	MatchBG     color.Color
	CurrentFG   color.Color // the match the cursor is on
	CurrentBG   color.Color
	SelectedFG  color.Color // text selected for copying
	SelectedBG  color.Color
	UserFG      color.Color // the "you" label of a chat message
	UserBG      color.Color
	AgentFG     color.Color // the "snow" label of a reply
	AgentBG     color.Color
	Queued      color.Color // messages that wait for the running turn; on the terminal's background
	ActivityFG  color.Color // the line that says a turn is running
	ActivityBG  color.Color
	Added       color.Color // lines a change adds; on the terminal's background
	Removed     color.Color // lines a change removes; on the terminal's background
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
	Caret                                      string // ends text being typed
	TopLeft, TopRight, BottomLeft, BottomRight string
	// Spinner holds the frames, one character each, that turn while an
	// agent works.
	Spinner string
	// Collapsed and Expanded mark a tool call whose details are hidden
	// or shown.
	Collapsed, Expanded string
	// Mask is the glyph that hides each character of a secret.
	Mask string
	// Checked and Unchecked mark an option of a question that takes more
	// than one answer, and a task that is done or still to do. Working
	// marks the task in progress.
	Checked, Unchecked, Working string
	// AgentIdle, AgentWorking, and AgentNeedsYou follow a window's name in
	// the status bar while an agent in it waits for a task, runs a turn,
	// or waits for the user. One cell each.
	AgentIdle, AgentWorking, AgentNeedsYou string
}

// Default returns the default theme, Frost.
func Default() Theme { return frost() }

// defaultGlyphs are the box-drawing characters and markers every built-in
// theme uses.
func defaultGlyphs() Glyphs {
	return Glyphs{
		Vertical: "│", Horizontal: "─",
		Cross: "┼", TeeLeft: "┤", TeeRight: "├", TeeUp: "┴", TeeDown: "┬",
		Zoomed: "Z", ActiveTab: "*",
		SessionLeft: "[", SessionRight: "]",
		Pointer: "> ", Caret: "_",
		TopLeft: "┌", TopRight: "┐", BottomLeft: "└", BottomRight: "┘",
		Spinner:   "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏",
		Collapsed: "▸", Expanded: "▾",
		Mask:    "•",
		Checked: "[x]", Unchecked: "[ ]", Working: "[>]",
		AgentIdle: "○", AgentWorking: "◐", AgentNeedsYou: "●",
	}
}

// Resolve returns t with every token it leaves empty taken from Default, so
// a theme from the config file may set only the tokens it changes.
func (t Theme) Resolve() Theme {
	d := Default()
	fill := func(c *color.Color, def color.Color) {
		if *c == nil {
			*c = def
		}
	}
	fill(&t.Border, d.Border)
	fill(&t.FocusBorder, d.FocusBorder)
	fill(&t.BarFG, d.BarFG)
	fill(&t.BarBG, d.BarBG)
	fill(&t.TabActive, d.TabActive)
	fill(&t.TabActiveBG, d.TabActiveBG)
	fill(&t.Accent, d.Accent)
	fill(&t.Muted, d.Muted)
	fill(&t.Success, d.Success)
	fill(&t.Error, d.Error)
	fill(&t.ErrorFG, d.ErrorFG)
	fill(&t.PromptFG, d.PromptFG)
	fill(&t.PromptBG, d.PromptBG)
	fill(&t.Selection, d.Selection)
	fill(&t.SelectionFG, d.SelectionFG)
	fill(&t.IndicatorFG, d.IndicatorFG)
	fill(&t.IndicatorBG, d.IndicatorBG)
	fill(&t.MatchFG, d.MatchFG)
	fill(&t.MatchBG, d.MatchBG)
	fill(&t.CurrentFG, d.CurrentFG)
	fill(&t.CurrentBG, d.CurrentBG)
	fill(&t.SelectedFG, d.SelectedFG)
	fill(&t.SelectedBG, d.SelectedBG)
	fill(&t.UserFG, d.UserFG)
	fill(&t.UserBG, d.UserBG)
	fill(&t.AgentFG, d.AgentFG)
	fill(&t.AgentBG, d.AgentBG)
	fill(&t.Queued, d.Queued)
	fill(&t.ActivityFG, d.ActivityFG)
	fill(&t.ActivityBG, d.ActivityBG)
	fill(&t.Added, d.Added)
	fill(&t.Removed, d.Removed)
	if t.Glyphs == (Glyphs{}) {
		t.Glyphs = d.Glyphs
	}
	return t
}

// colors returns every color token by name, for checks over all of them.
func (t Theme) colors() map[string]color.Color {
	return map[string]color.Color{
		"Border": t.Border, "FocusBorder": t.FocusBorder, "BarFG": t.BarFG, "BarBG": t.BarBG,
		"TabActive": t.TabActive, "TabActiveBG": t.TabActiveBG, "Accent": t.Accent, "Muted": t.Muted,
		"Success": t.Success, "Error": t.Error, "ErrorFG": t.ErrorFG, "PromptFG": t.PromptFG, "PromptBG": t.PromptBG,
		"Selection": t.Selection, "SelectionFG": t.SelectionFG, "IndicatorFG": t.IndicatorFG, "IndicatorBG": t.IndicatorBG,
		"MatchFG": t.MatchFG, "MatchBG": t.MatchBG, "CurrentFG": t.CurrentFG, "CurrentBG": t.CurrentBG,
		"SelectedFG": t.SelectedFG, "SelectedBG": t.SelectedBG,
		"UserFG": t.UserFG, "UserBG": t.UserBG, "AgentFG": t.AgentFG, "AgentBG": t.AgentBG,
		"Queued": t.Queued, "ActivityFG": t.ActivityFG, "ActivityBG": t.ActivityBG,
		"Added": t.Added, "Removed": t.Removed,
	}
}

// ErrUnknownTheme means a theme name that Snow does not define.
var ErrUnknownTheme = errors.New("unknown theme")

// Named returns the theme called name, as set in the config file: a
// built-in theme's name, ignoring case, or "default" for Frost.
func Named(name string) (Theme, error) {
	if strings.EqualFold(name, "default") {
		return Default(), nil
	}
	if t, ok := ByName(name); ok {
		return t, nil
	}
	names := make([]string, 0, len(Builtin()))
	for _, t := range Builtin() {
		names = append(names, t.Name)
	}
	return Theme{}, fmt.Errorf("%w %q; the available themes are: default, %s", ErrUnknownTheme, name, strings.Join(names, ", "))
}
