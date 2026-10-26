package chat

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/emmyfong/snow/internal/client/command"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
)

// commandDescribeWidth is the narrowest pane that has room for a command's
// description beside its name.
const commandDescribeWidth = 48

// commandView is the command list as drawn: its rows, which item is on each
// row, and where the list ends.
type commandView struct {
	list  command.List
	lines []string
	rows  []int // the absolute item index on each line
}

// commandLimit is the most rows the list may take, leaving the input and
// the transcript room. It is 0 when the pane is too short for a list.
func (m *Model) commandLimit() int {
	if !m.cmdOpen || m.height <= 0 {
		return 0
	}
	avail := m.height - ui.TextAreaRows(m.inputOptions()) - 1 - m.barRows()
	if avail <= 0 {
		return 0
	}
	return min(max(m.height/3, 3), avail)
}

// commandView builds the list for the current input and panes.
func (m *Model) commandView() (commandView, bool) {
	if !m.cmdOpen {
		return commandView{}, false
	}
	limit := m.commandLimit()
	if limit <= 0 {
		return commandView{}, false
	}
	l := command.NewList(m.commandMatches(), true, m.width >= commandDescribeWidth)
	if len(l.Items) == 0 {
		line := lipgloss.NewStyle().Foreground(m.th.Muted).Render(ansi.Truncate("No command matches.", m.width, "…"))
		line += strings.Repeat(" ", max(m.width-ansi.StringWidth(line), 0))
		return commandView{list: l, lines: []string{line}}, true
	}
	sel := l.Selected(m.cmdSel)
	view, rows := ui.ListRows(m.th, ui.ListOptions{Items: l.Items, Selected: sel, Width: m.width, Height: limit})
	return commandView{list: l, lines: strings.Split(view, "\n"), rows: rows}, true
}

// commandHeight is how many rows the list takes.
func (m *Model) commandHeight() int {
	v, ok := m.commandView()
	if !ok {
		return 0
	}
	return len(v.lines)
}

// commandTop is the first screen row of the list.
func (m *Model) commandTop() int { return m.barRows() + m.transcriptHeight() + 1 }

// commandClick handles a click at y when it lands on a command row. It
// runs a command that takes no argument, or completes one that does.
func (m *Model) commandClick(y int) *api.ChatInput {
	v, ok := m.commandView()
	if !ok {
		return nil
	}
	row := y - m.commandTop()
	if row < 0 || row >= len(v.lines) || row >= len(v.rows) {
		return nil
	}
	item := v.rows[row]
	if item < 0 || item >= len(v.list.ItemOf) {
		return nil
	}
	ci := v.list.ItemOf[item]
	if ci < 0 || ci >= len(v.list.Commands) {
		return nil
	}
	c := v.list.Commands[ci]
	m.cmdSel = ci
	if c.Args == "" {
		return m.run(c, "")
	}
	m.complete(c)
	return nil
}
