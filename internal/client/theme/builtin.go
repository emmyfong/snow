package theme

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

// Builtin returns the themes that ship with Snow, the default first. Most
// follow a well-known palette: Windows Terminal (Campbell, One Half Dark),
// VS Code (Dark+), Solarized, Dracula, Nord, Gruvbox, Tokyo Night, Catppuccin,
// and OpenCode. Clay is inspired by Claude Code's colors; Anthropic's terms do
// not allow that name in another product's feature names.
func Builtin() []Theme {
	return []Theme{frost(), campbell(), oneHalfDark(), darkPlus(), solarizedDark(), dracula(), nord(), gruvboxDark(), tokyoNight(), catppuccinMocha(), openCode(), clay()}
}

// ByName returns the built-in theme with name, ignoring case.
func ByName(name string) (Theme, bool) {
	for _, t := range Builtin() {
		if strings.EqualFold(t.Name, name) {
			return t, true
		}
	}
	return Theme{}, false
}

// palette builds a theme from hex colors in a fixed order, so each built-in
// theme reads as one line of its palette.
func palette(name, border, focus, markBG, markFG, currentBG, currentFG, promptBG, promptFG, accent, muted, success, errColor, selection string) Theme {
	c := lipgloss.Color
	onAccent := ink(colorprofile.TrueColor, c(accent))
	return Theme{
		// The status bar is the accent, and the shown window's tab is the
		// bar inverted, so the current window is a block of its own color.
		Name: name, Border: c(border), FocusBorder: c(focus), BarBG: c(accent), BarFG: onAccent,
		TabActiveBG: onAccent, TabActive: c(accent), PromptBG: c(promptBG), PromptFG: c(promptFG),
		Accent: c(accent), Muted: c(muted), Success: c(success), Error: c(errColor),
		ErrorFG: ink(colorprofile.TrueColor, c(errColor)), Selection: c(selection),
		SelectionFG: ink(colorprofile.TrueColor, c(selection)),
		// Copy mode has its own marker and current-match pairs; matches look
		// like a question.
		IndicatorFG: c(markFG), IndicatorBG: c(markBG),
		MatchFG: c(promptFG), MatchBG: c(promptBG),
		CurrentFG: c(currentFG), CurrentBG: c(currentBG),
		SelectedFG: ink(colorprofile.TrueColor, c(selection)), SelectedBG: c(selection),
		Glyphs: defaultGlyphs(),
	}
}

func frost() Theme {
	return palette("Frost", "#3b4a5a", "#7fb8e6", "#1e2a36", "#c9d7e3", "#7fb8e6", "#0e1620", "#cfe8ff", "#0e1620", "#7fb8e6", "#6b7c8c", "#8fd19e", "#e07a7a", "#2d4a66")
}

func campbell() Theme {
	return palette("Campbell", "#767676", "#13a10e", "#13a10e", "#0c0c0c", "#0c0c0c", "#f2f2f2", "#3a96dd", "#0c0c0c", "#3a96dd", "#767676", "#13a10e", "#c50f1f", "#0037da")
}

func oneHalfDark() Theme {
	return palette("One Half Dark", "#5a6374", "#61afef", "#3e4451", "#dcdfe4", "#61afef", "#282c34", "#c678dd", "#282c34", "#61afef", "#5a6374", "#98c379", "#e06c75", "#3e4451")
}

func darkPlus() Theme {
	return palette("Dark+", "#3c3c3c", "#007acc", "#007acc", "#ffffff", "#ffffff", "#007acc", "#cc6633", "#ffffff", "#569cd6", "#808080", "#6a9955", "#f44747", "#264f78")
}

func solarizedDark() Theme {
	return palette("Solarized Dark", "#586e75", "#268bd2", "#073642", "#93a1a1", "#268bd2", "#002b36", "#cb4b16", "#fdf6e3", "#2aa198", "#586e75", "#859900", "#dc322f", "#073642")
}

func dracula() Theme {
	return palette("Dracula", "#6272a4", "#bd93f9", "#44475a", "#f8f8f2", "#bd93f9", "#282a36", "#ff79c6", "#282a36", "#8be9fd", "#6272a4", "#50fa7b", "#ff5555", "#44475a")
}

func nord() Theme {
	return palette("Nord", "#4c566a", "#88c0d0", "#3b4252", "#d8dee9", "#88c0d0", "#2e3440", "#ebcb8b", "#2e3440", "#81a1c1", "#616e88", "#a3be8c", "#bf616a", "#434c5e")
}

func gruvboxDark() Theme {
	return palette("Gruvbox Dark", "#665c54", "#fabd2f", "#3c3836", "#ebdbb2", "#fabd2f", "#282828", "#fe8019", "#282828", "#83a598", "#928374", "#b8bb26", "#fb4934", "#504945")
}

func tokyoNight() Theme {
	return palette("Tokyo Night", "#3b4261", "#7aa2f7", "#24283b", "#a9b1d6", "#7aa2f7", "#1a1b26", "#bb9af7", "#1a1b26", "#7dcfff", "#565f89", "#9ece6a", "#f7768e", "#283457")
}

func catppuccinMocha() Theme {
	return palette("Catppuccin Mocha", "#45475a", "#cba6f7", "#313244", "#cdd6f4", "#cba6f7", "#1e1e2e", "#fab387", "#1e1e2e", "#89b4fa", "#6c7086", "#a6e3a1", "#f38ba8", "#45475a")
}

func openCode() Theme {
	return palette("OpenCode", "#484848", "#fab283", "#141414", "#eeeeee", "#fab283", "#0a0a0a", "#9d7cd8", "#0a0a0a", "#5c9cf5", "#808080", "#7fd88f", "#e06c75", "#303030")
}

func clay() Theme {
	return palette("Clay", "#4a4743", "#d77757", "#2b2a27", "#e8e6e3", "#d77757", "#1f1e1d", "#b1b9f9", "#1f1e1d", "#d77757", "#999999", "#4eba65", "#ff6b80", "#3d3a35")
}
