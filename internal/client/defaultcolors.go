package client

import (
	"fmt"
	"image/color"
	"io"
	"os"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

// paintsDefaultColors reports whether Snow can apply the theme's colors.
// Some hosts apply colors but ignore the reset. Require both reports so
// Snow can restore the colors it found when it opened.
func (a *app) paintsDefaultColors() bool {
	return !a.terminalBackground && a.profile != nil && a.profile.Profile >= colorprofile.ANSI &&
		a.originalForeground != nil && a.originalBackground != nil
}

func (a *app) trackDefaultColors() {
	a.paintedDefaults = a.paintedDefaults || a.paintsDefaultColors()
	for _, screen := range []Screen{a.screen, a.back} {
		if screen != nil {
			a.setScreenBackground(screen)
		}
	}
}

func (a *app) setScreenBackground(s Screen) {
	if screen, ok := s.(interface{ SetPaintedBackground(color.Color) }); ok {
		var bg color.Color
		if a.paintsDefaultColors() {
			bg = a.theme.Background
		}
		screen.SetPaintedBackground(bg)
	}
}

// defaultColors returns the colors Bubble Tea applies to the terminal.
func (a *app) defaultColors() (bg, fg color.Color) {
	if a.paintsDefaultColors() {
		th := a.theme.For(a.profile.Profile)
		return th.Background, th.Text
	}
	if a.terminalBackground && a.paintedDefaults {
		return a.originalBackground, a.originalForeground
	}
	return nil, nil
}

// restoreDefaultColors follows the renderer's reset, which restores the
// terminal profile and can lose colors applied by the shell before Snow.
func (a *app) restoreDefaultColors(out io.Writer) error {
	if !a.paintedDefaults {
		return nil
	}
	if out == nil {
		out = os.Stdout
	}
	format := func(c color.Color) string {
		r, g, b, _ := c.RGBA()
		return fmt.Sprintf("rgb:%04x/%04x/%04x", r, g, b)
	}
	return writeTerminalSequence(out,
		ansi.SetForegroundColor(format(a.originalForeground))+ansi.SetBackgroundColor(format(a.originalBackground)))
}
