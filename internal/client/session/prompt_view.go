package session

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/ui"
)

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
