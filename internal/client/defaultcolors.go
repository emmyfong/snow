package client

import "image/color"

// defaultColors returns the colors Bubble Tea applies to the terminal.
// Some hosts apply the colors but ignore the reset when Snow exits.
// Leave their defaults alone when they do not report both colors.
func (a *app) defaultColors() (bg, fg color.Color) {
	if a.terminalBackground || a.profile == nil || !a.foregroundReported || !a.backgroundReported {
		return nil, nil
	}
	th := a.theme.For(a.profile.Profile)
	return th.Background, th.Text
}
