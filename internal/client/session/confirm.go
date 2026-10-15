package session

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/pkg/api"
)

// confirmation is a question in the status bar that guards a command.
type confirmation struct {
	question string
	cmd      api.Command
}

// confirmClosePane asks before closing the focused pane, as tmux does:
// closing ends the program in it.
// Pane IDs only grow, so the question names the pane by its program,
// folder, and place in the window instead.
func (m *Model) confirmClosePane() {
	p, w := m.focused(), m.shown()
	if p == nil || w == nil {
		return
	}
	place := 1
	for i := range w.Panes {
		if w.Panes[i].ID == p.ID {
			place = i + 1
		}
	}
	what := strings.TrimSpace(p.Profile + " " + p.Folder)
	if what != "" {
		what += ", "
	}
	m.confirm = &confirmation{
		question: fmt.Sprintf("close this pane (%s%d of %d)? y/n", what, place, len(w.Panes)),
		cmd:      api.Command{Op: api.OpClose, Target: p.ID},
	}
}

// confirmCloseWindow asks before closing the shown window and its panes.
func (m *Model) confirmCloseWindow() {
	w := m.shown()
	if w == nil {
		return
	}
	name := ""
	if w.Name != "" {
		name = " " + strconv.Quote(w.Name)
	}
	panes := "1 pane"
	if len(w.Panes) != 1 {
		panes = strconv.Itoa(len(w.Panes)) + " panes"
	}
	m.confirm = &confirmation{
		question: fmt.Sprintf("close window %d%s and its %s? y/n", w.Index, name, panes),
		cmd:      api.Command{Op: api.OpCloseWindow, Target: w.Index},
	}
}

// answer handles the key that answers the open question: y runs the
// command, any other key cancels it.
func (m *Model) answer(k tea.Key) {
	c := m.confirm
	m.confirm = nil
	if k.Text == "y" || k.Text == "Y" {
		m.send(&c.cmd)
	}
}
