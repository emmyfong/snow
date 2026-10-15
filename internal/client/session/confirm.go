package session

import (
	"fmt"
	"strconv"

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
func (m *Model) confirmClosePane() {
	p := m.focused()
	if p == nil {
		return
	}
	m.confirm = &confirmation{
		question: fmt.Sprintf("close pane %d? (y/n)", p.ID),
		cmd:      api.Command{Op: api.OpClose, Target: p.ID},
	}
}

// confirmCloseWindow asks before closing the shown window and its panes.
func (m *Model) confirmCloseWindow() {
	w := m.shown()
	if w == nil {
		return
	}
	m.confirm = &confirmation{
		question: "close window " + strconv.Itoa(w.Index) + "? (y/n)",
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
