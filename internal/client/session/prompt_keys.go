package session

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/command"
	"github.com/emmyfong/snow/internal/client/keys"
)

// promptKey edits the typed text, moves in the list, and runs the command
// on Enter.
func (m *Model) promptKey(k tea.Key) tea.Cmd {
	p := m.prompt
	switch k.Code {
	case tea.KeyEscape:
		m.prompt = nil
	case tea.KeyEnter:
		return m.promptEnter()
	case tea.KeyUp:
		p.sel = max(p.sel-1, 0)
	case tea.KeyDown:
		p.sel = min(p.sel+1, max(p.rows()-1, 0))
	case tea.KeyTab:
		if ms := p.commandMatches(); p.mode == modeCommands && p.sel < len(ms) {
			p.complete(ms[p.sel])
		}
	default:
		if s, ok := keys.Edit(p.input, k); ok {
			p.input, p.sel, p.notice = s, 0, ""
		}
	}
	return nil
}

// complete puts command c in the input, ready for its argument.
func (p *prompting) complete(c command.Command) {
	p.input, p.sel = c.Name, 0
	if c.Args != "" {
		p.input += " "
	}
}

// promptEnter runs what the popup holds: the command named, else the
// highlighted one; the new name; or the highlighted theme.
func (m *Model) promptEnter() tea.Cmd {
	p := m.prompt
	switch p.mode {
	case modeName:
		return m.renameTo(p.input)
	case modeThemes:
		ts := p.themeMatches()
		if p.sel >= len(ts) {
			p.notice = "No theme matches."
			return nil
		}
		m.prompt = nil
		return useTheme(ts[p.sel].Name)
	}
	word, rest := command.Split(p.input)
	c, ok := command.Find(word, command.InSession)
	if ms := p.commandMatches(); !ok && p.sel < len(ms) {
		c, ok = ms[p.sel], true
	}
	if !ok {
		p.notice = "Unknown command " + word + "."
		return nil
	}
	return commandRuns[c.Name](m, rest)
}

// promptMouse handles a mouse event while the popup is open. A click on a
// command runs it when its arguments are optional, else completes it; a
// click on a theme uses it; a click outside closes the popup.
func (m *Model) promptMouse(msg tea.MouseMsg) tea.Cmd {
	click, ok := msg.(tea.MouseClickMsg)
	if !ok {
		return nil
	}
	l, ok := m.promptLayout()
	if click.Button != tea.MouseLeft || !ok || click.X < l.x || click.X >= l.x+l.w || click.Y < l.y || click.Y >= l.y+l.h {
		m.prompt = nil
		return nil
	}
	i := click.Y - l.listY
	if click.X <= l.x || click.X >= l.x+l.w-1 || i < 0 || i >= l.count {
		return nil
	}
	p := m.prompt
	p.sel = l.first + i
	if p.mode == modeCommands {
		c := p.commandMatches()[p.sel]
		if c.Args == "" || strings.HasPrefix(c.Args, "[") {
			return commandRuns[c.Name](m, "")
		}
		p.complete(c)
		return nil
	}
	return m.promptEnter()
}
