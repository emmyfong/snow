package session

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/command"
	"github.com/emmyfong/snow/internal/client/keys"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/internal/client/ui"
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
	mode   promptMode
	input  string
	sel    int    // the highlighted row
	notice string // why the last try was refused
}

// openPrompt shows the command popup over the window.
func (m *Model) openPrompt() { m.prompt = &prompting{} }

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

// promptLayout is where the popup goes and which rows its list shows.
type promptLayout struct {
	box          string
	x, y, w, h   int
	listY        int // the screen row of the list's first row
	first, count int // the first list row shown, and how many are shown
}

// promptChrome are the popup's rows besides the list: two frame rows, the
// input, the notice, a blank, and the key hints.
const promptChrome = 6

// maxPromptName bounds the session name in the rename prompt, so a long
// name leaves room for the new one.
const maxPromptName = 20

func (m *Model) promptLayout() (promptLayout, bool) {
	th, p := m.theme, m.prompt
	cols, rows := m.cols, m.windowRows()
	w := min(64, cols-4)
	if w < 20 || rows < promptChrome+1 {
		return promptLayout{}, false
	}
	inner := w - 2
	muted := lipgloss.NewStyle().Foreground(th.Muted)
	title, prompt := "Commands", ": "
	var items []ui.ListItem
	var hints []ui.Hint
	empty := ""
	switch p.mode {
	case modeCommands:
		for _, c := range p.commandMatches() {
			items = append(items, ui.ListItem{Cols: []string{c.Line(), c.Desc}})
		}
		hints = []ui.Hint{{Key: "enter", Desc: "run"}, {Key: "tab", Desc: "complete"}, {Key: "↑↓", Desc: "select"}, {Key: "esc", Desc: "close"}}
		empty = "No command matches."
	case modeThemes:
		title, prompt = "Themes", "theme: "
		for _, t := range p.themeMatches() {
			note := ""
			if t.Name == m.base.Name {
				note = "in use"
			}
			items = append(items, ui.ListItem{Cols: []string{t.Name, note}})
		}
		hints = []ui.Hint{{Key: "enter", Desc: "use"}, {Key: "↑↓", Desc: "select"}, {Key: "esc", Desc: "close"}}
		empty = "No theme matches."
	case modeName:
		title, prompt = "Rename session", "New name for "+ansi.Truncate(m.current(), maxPromptName, "…")+": "
		hints = []ui.Hint{{Key: "enter", Desc: "rename"}, {Key: "ctrl+u", Desc: "clear"}, {Key: "esc", Desc: "cancel"}}
	}
	l := promptLayout{count: min(len(items), rows-promptChrome)}
	body := []string{" " + ui.Input(th, ui.InputOptions{Value: p.input, Prompt: prompt, Width: inner - 1, Focused: true})}
	notice := ""
	if p.notice != "" {
		notice = " " + lipgloss.NewStyle().Foreground(th.Error).Render(ansi.Truncate(p.notice, inner-1, "…"))
	}
	body = append(body, notice)
	switch {
	case l.count > 0:
		sel := min(p.sel, len(items)-1)
		l.first = min(max(sel-l.count+1, 0), len(items)-l.count)
		list := ui.List(th, ui.ListOptions{Items: items[l.first : l.first+l.count], Selected: sel - l.first, Width: inner})
		body = append(body, strings.Split(list, "\n")...)
	case empty != "":
		body = append(body, " "+muted.Render(empty))
	}
	body = append(body, "", " "+ui.KeyHints(th, hints, inner-1))
	l.box = ui.Panel(th, ui.PanelOptions{Title: title, Body: strings.Join(body, "\n"), Width: w})
	l.w, l.h = ui.Size(l.box)
	l.x, l.y = max((cols-l.w)/2, 0), max((rows-l.h)/2, 0)
	l.listY = l.y + 3
	return l, true
}

// overlayPrompt draws the open popup over content, the window area of rows
// rows.
func (m *Model) overlayPrompt(content string) string {
	if m.prompt == nil {
		return content
	}
	l, ok := m.promptLayout()
	if !ok {
		return content
	}
	return ui.Overlay(content, l.box, l.x, l.y)
}
