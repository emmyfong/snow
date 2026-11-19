package session

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/command"
	"github.com/emmyfong/snow/internal/client/keys"
	"github.com/emmyfong/snow/internal/client/themepick"
)

// promptKey edits the typed text, moves in the list, and runs the command
// on Enter.
func (m *Model) promptKey(k tea.Key) tea.Cmd {
	p := m.prompt
	switch k.Code {
	case tea.KeyEscape:
		m.closePrompt(false)
	case tea.KeyEnter:
		return m.promptEnter()
	case tea.KeyUp:
		if p.mode == modeThemes && p.themePick != nil {
			m.previewTheme(p.themePick.Move(-1))
			p.sel = p.themePick.Selected()
		} else {
			p.sel = max(p.sel-1, 0)
		}
	case tea.KeyDown:
		if p.mode == modeThemes && p.themePick != nil {
			m.previewTheme(p.themePick.Move(1))
			p.sel = p.themePick.Selected()
		} else {
			p.sel = min(p.sel+1, max(p.rows()-1, 0))
		}
	case tea.KeyTab:
		if ms := p.commandMatches(); p.mode == modeCommands && p.sel < len(ms) {
			p.complete(ms[p.sel])
		}
	default:
		if s, ok := keys.Edit(p.input, k); ok {
			p.input, p.sel, p.notice = s, 0, ""
			if p.mode == modeThemes && p.themePick != nil {
				before := p.themePick.Restore()
				p.themePick = themepick.New(p.themeMatches(), before)
				p.sel = p.themePick.Selected()
				m.previewTheme(p.themePick.Shown())
			}
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
		if p.themePick == nil || len(p.themePick.Items()) == 0 {
			p.notice = "No theme matches."
			return nil
		}
		th := p.themePick.Keep()
		m.closePrompt(true)
		return useTheme(th.Name)
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
	y := click.Y - m.areaTop()
	if click.Button != tea.MouseLeft || !ok || click.X < l.x || click.X >= l.x+l.w || y < l.y || y >= l.y+l.h {
		m.closePrompt(false)
		return nil
	}
	i := y - l.listY
	if click.X <= l.x || click.X >= l.x+l.w-1 || i < 0 || i >= l.count {
		return nil
	}
	p := m.prompt
	p.sel = l.first + i
	if p.mode == modeThemes && p.themePick != nil {
		m.previewTheme(p.themePick.Move(p.sel - p.themePick.Selected()))
	}
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
