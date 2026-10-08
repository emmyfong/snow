package main

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/xpty"
)

// frameInterval caps redraws at about 30 per second, so a flood of output
// cannot starve key handling.
const frameInterval = 33 * time.Millisecond

type redrawMsg struct{}

type exitedMsg struct{ err error }

// pane is the Bubble Tea model: one shell drawn full screen.
type pane struct {
	*shell
	err error
}

func newPane(name string, args []string, width, height int) (*pane, error) {
	s, err := startShell(name, args, width, height)
	if err != nil {
		return nil, err
	}
	return &pane{shell: s}, nil
}

func (p *pane) Init() tea.Cmd {
	return tea.Batch(redraw(), p.waitExit())
}

func redraw() tea.Cmd {
	return tea.Tick(frameInterval, func(time.Time) tea.Msg { return redrawMsg{} })
}

// waitExit uses xpty.WaitProcess because exec.Cmd.Wait never returns for
// ConPTY processes on Windows.
func (p *pane) waitExit() tea.Cmd {
	return func() tea.Msg {
		return exitedMsg{err: xpty.WaitProcess(context.Background(), p.cmd)}
	}
}

func (p *pane) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		p.emu.SendKey(uv.KeyPressEvent(msg))
	case tea.PasteMsg:
		p.emu.Paste(msg.Content)
	case tea.WindowSizeMsg:
		if err := p.resize(msg.Width, msg.Height); err != nil {
			p.err = err
			return p, tea.Quit
		}
	case redrawMsg:
		return p, redraw()
	case exitedMsg:
		p.err = msg.err
		return p, tea.Quit
	}
	return p, nil
}

func (p *pane) View() tea.View {
	v := tea.NewView(p.emu.Render())
	v.AltScreen = true
	pos := p.emu.CursorPosition()
	v.Cursor = tea.NewCursor(pos.X, pos.Y)
	return v
}
