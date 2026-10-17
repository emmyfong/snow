package client

import (
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/config"
)

// themeCheckEvery is how often a client looks at its config file for a new
// theme, so a theme picked in one Snow window reaches the others.
const themeCheckEvery = 2 * time.Second

// configCheckedMsg is the result of one look at the config file: its time
// of change, and the theme it names when that changed and is valid.
type configCheckedMsg struct {
	th  *theme.Theme
	mod time.Time
}

// themed is a screen that can change its theme while it is open.
type themed interface{ SetTheme(theme.Theme) }

// watchTheme looks at the config file at path after themeCheckEvery. The
// look runs off the model's goroutine, as a tea.Tick does.
func watchTheme(path string, last time.Time) tea.Cmd {
	return tea.Tick(themeCheckEvery, func(time.Time) tea.Msg { return checkTheme(path, last) })
}

// checkTheme reads the theme from the config file at path if the file
// changed after last. A missing, unreadable, or wrong file names no theme:
// the screen keeps what it has, and the next start reports the problem.
func checkTheme(path string, last time.Time) configCheckedMsg {
	fi, err := os.Stat(path)
	if err != nil || !fi.ModTime().After(last) {
		return configCheckedMsg{mod: last}
	}
	res := configCheckedMsg{mod: fi.ModTime()}
	cfg, err := config.Load(path)
	if err != nil {
		return res
	}
	if th, err := theme.Named(cfg.Theme.Name); err == nil {
		res.th = &th
	}
	return res
}

// applyTheme makes th the app's theme and the open screen's.
func (a *app) applyTheme(th theme.Theme) {
	a.theme = th
	if s, ok := a.screen.(themed); ok {
		s.SetTheme(th)
	}
}
