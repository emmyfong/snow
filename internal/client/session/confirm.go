package session

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/confirm"
	"github.com/emmyfong/snow/pkg/api"
)

// confirmation is a popup that guards a request: a command, or the end of
// a session.
type confirmation struct {
	popup  *confirm.Model
	msg    api.Message
	altMsg api.Message // sent for the popup's second action, if it has one
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
	if p.Worktree != "" {
		m.confirmCloseAgent(p)
		return
	}
	what := strings.TrimSpace(p.Profile + " " + p.Folder)
	if what != "" {
		what += ", "
	}
	if p.Purpose == "sign-in" {
		m.confirm = &confirmation{popup: confirm.New("Cancel sign-in", "Close this sign-in attempt? Snow will check the connection afterward.", "Cancel sign-in"),
			msg: &api.Command{Op: api.OpClose, Target: p.ID}}
		return
	}
	if runtime, ok := m.own[p.ID].(runtimeChooser); ok && runtime.RuntimeInfo() != nil && p.AgentState != api.AgentIdle {
		m.confirm = &confirmation{popup: confirm.New("Stop and close chat", "Stop this run and its owned child tasks, then close? Saved progress and paused drafts stay available.", "Stop and close"),
			msg: &api.Command{Op: api.OpClose, Target: p.ID}}
		return
	}
	m.confirm = &confirmation{
		popup: confirm.New("Close pane", fmt.Sprintf("Close this pane (%s%d of %d)?", what, place, len(w.Panes)), "Close"),
		msg:   &api.Command{Op: api.OpClose, Target: p.ID},
	}
}

// confirmCloseAgent asks before closing an agent that works in a worktree
// Snow made, and whether its worktree goes too. Its branch always stays.
func (m *Model) confirmCloseAgent(p *api.PaneInfo) {
	question := fmt.Sprintf("Close this agent on branch %s? The branch stays. Keep its worktree, or remove it? A worktree with uncommitted changes is kept.", p.Branch)
	m.confirm = &confirmation{
		popup:  confirm.NewChoice("Close agent", question, "Keep worktree", "Remove worktree", "r"),
		msg:    &api.Command{Op: api.OpClose, Target: p.ID},
		altMsg: &api.Command{Op: api.OpClose, Target: p.ID, Args: map[string]string{api.ArgWorktree: api.WorktreeRemove}},
	}
}

// confirmCloseWindow asks before closing the shown window and its panes.
func (m *Model) confirmCloseWindow() {
	if w := m.shown(); w != nil {
		m.confirmCloseWindowAt(w.Index)
	}
}

// confirmCloseWindowAt asks before closing the window with the given index.
func (m *Model) confirmCloseWindowAt(index int) {
	if m.layout == nil {
		return
	}
	for _, w := range m.layout.Windows {
		if w.Index != index {
			continue
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
			popup: confirm.New("Close window", fmt.Sprintf("Close window %d%s and its %s?", w.Index, name, panes), "Close"),
			msg:   &api.Command{Op: api.OpCloseWindow, Target: w.Index},
		}
		return
	}
}

// confirmKill asks before it ends session name and its programs.
func (m *Model) confirmKill(name string) {
	m.confirm = &confirmation{
		popup: confirm.New("Kill session", fmt.Sprintf("Kill session %s and its programs?", name), "Kill"),
		msg:   &api.KillSession{Session: name},
	}
}

// confirmKey sends a key to the open popup and runs the command on Yes.
func (m *Model) confirmKey(k tea.Key) {
	m.confirmResult(m.confirm.popup.Key(k))
}

// confirmMouse handles a mouse event while the popup is open: only a left
// click matters.
func (m *Model) confirmMouse(msg tea.MouseMsg) {
	if click, ok := msg.(tea.MouseClickMsg); ok && click.Button == tea.MouseLeft {
		m.confirmResult(m.confirm.popup.Click(m.theme, m.cols, m.windowRows(), click.X, click.Y))
	}
}

func (m *Model) confirmResult(r confirm.Result) {
	c := m.confirm
	switch r {
	case confirm.Yes:
		m.confirm = nil
		m.send(c.msg)
	case confirm.Alt:
		m.confirm = nil
		if c.altMsg != nil {
			m.send(c.altMsg)
		}
	case confirm.No:
		m.confirm = nil
	}
}

// overlayConfirm draws the open popup over content, the window area of rows
// rows.
func (m *Model) overlayConfirm(content string, rows int) string {
	if m.confirm == nil {
		return content
	}
	return m.overlayPopup(content, m.confirm.popup, rows)
}
