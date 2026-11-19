package session

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/command"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/themepick"
)

// promptMode is what the command popup asks for.
type promptMode int

const (
	modeCommands promptMode = iota // a command and its argument
	modeName                       // the new name for this session
	modeThemes                     // a theme from the list
)

// prompting is the open command popup, as tmux's command prompt: a typed
// command with a list of the commands that match.
type prompting struct {
	mode      promptMode
	input     string
	sel       int    // the highlighted row
	notice    string // why the last try was refused
	themePick *themepick.Model
}

// openPrompt shows the command popup over the window. A window too small
// for it gets a note instead: a popup that cannot be drawn must not take
// the keys.
func (m *Model) openPrompt() tea.Cmd {
	m.closePrompt(false)
	m.prompt = &prompting{}
	if _, ok := m.promptLayout(); !ok {
		m.prompt = nil
		return m.ShowError("Window too small for the command list")
	}
	return nil
}

// commandMatches are the session commands that match the typed name.
func (p *prompting) commandMatches() []command.Command {
	word, _ := command.Split(p.input)
	return command.Match(word, command.InSession)
}

// themeMatches are the themes whose names contain the typed text.
func (p *prompting) themeMatches() []theme.Theme {
	q := strings.ToLower(strings.TrimSpace(p.input))
	var out []theme.Theme
	for _, t := range theme.Builtin() {
		if strings.Contains(strings.ToLower(t.Name), q) {
			out = append(out, t)
		}
	}
	return out
}

func (m *Model) previewTheme(th theme.Theme) {
	if m.profile != nil {
		th = th.ForBackground(*m.profile, m.paintedBackground)
	}
	m.applyTheme(th)
}

func (m *Model) closePrompt(keep bool) {
	if m.prompt == nil {
		return
	}
	if p := m.prompt.themePick; p != nil {
		if keep {
			m.previewTheme(p.Keep())
		} else {
			m.previewTheme(p.Restore())
		}
	}
	m.prompt = nil
}

// rows is how many rows the popup's list has.
func (p *prompting) rows() int {
	switch p.mode {
	case modeCommands:
		return len(p.commandMatches())
	case modeThemes:
		return len(p.themeMatches())
	}
	return 0
}
