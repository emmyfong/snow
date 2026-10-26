package session

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/chat"
	"github.com/emmyfong/snow/internal/client/command"
	"github.com/emmyfong/snow/internal/client/helppanel"
	"github.com/emmyfong/snow/internal/client/modepick"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/internal/client/resumepick"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// overlay is a popup over a chat pane that takes the keys and the clicks
// until it closes: the mode picker, the resume picker, and the help panel.
// A feature gives it three functions, so the session holds one field for
// all of them.
type overlay struct {
	popup
	pane  int // the chat pane it serves; it closes with the pane
	key   func(k tea.Key) picker.Result
	click func(th theme.Theme, cols, rows, x, y int) picker.Result
	// done runs once, when key or click returns anything but Open.
	done func(r picker.Result) tea.Cmd
}

// conversationPane is an own pane that has a conversation to resume into
// and a mode to set, such as a chat.
type conversationPane interface {
	Mode() string
	ModeInput(mode string) *api.ChatInput
	ResumeInput(conversation string) *api.ChatInput
}

// showOverlay opens o, or says the window is too small for what.
func (m *Model) showOverlay(o *overlay, what string) tea.Cmd {
	if box, _, _ := o.View(m.theme, m.cols, m.windowRows()); box == "" {
		return m.ShowError("Window too small for " + what)
	}
	m.over = o
	return nil
}

// overKey sends a key to the open overlay.
func (m *Model) overKey(k tea.Key) tea.Cmd { return m.overResult(m.over.key(k)) }

// overMouse handles the mouse while an overlay is open: a left click
// chooses or closes, a right click closes, and the rest is ignored.
func (m *Model) overMouse(msg tea.MouseMsg) tea.Cmd {
	click, ok := msg.(tea.MouseClickMsg)
	if !ok {
		return nil
	}
	if click.Button != tea.MouseLeft {
		return m.overResult(picker.Cancelled)
	}
	return m.overResult(m.over.click(m.theme, m.cols, m.windowRows(), click.X, click.Y))
}

// overResult closes the overlay on any result but Open, then lets it act.
func (m *Model) overResult(r picker.Result) tea.Cmd {
	if r == picker.Open {
		return nil
	}
	o := m.over
	m.over, m.resume = nil, nil
	if o.done == nil {
		return nil
	}
	return o.done(r)
}

// overlayOver draws the open overlay over content, the window area of rows
// rows.
func (m *Model) overlayOver(content string, rows int) string {
	if m.over == nil {
		return content
	}
	return m.overlayPopup(content, m.over, rows)
}

// openResume shows the picker of past conversations over chat pane id.
func (m *Model) openResume(id int) tea.Cmd {
	c, ok := m.own[id].(conversationPane)
	if !ok {
		return m.ShowError("/resume works in chat panes.")
	}
	pick := resumepick.New(id, m.now)
	o := &overlay{
		pane:  id,
		popup: pick,
		key: func(k tea.Key) picker.Result {
			r := pick.Key(k)
			if req := pick.Request(); req != nil {
				m.send(req)
			}
			return r
		},
		click: pick.Click,
		done: func(r picker.Result) tea.Cmd {
			if it, ok := pick.Choice(); ok && r == picker.Accepted {
				m.send(c.ResumeInput(it.ID))
			}
			return nil
		},
	}
	if cmd := m.showOverlay(o, "the conversation list"); m.over != o {
		return cmd
	}
	m.resume = pick
	m.send(pick.Request())
	return nil
}

// openModePicker shows the approval mode picker over chat pane id.
func (m *Model) openModePicker(id int) tea.Cmd {
	c, ok := m.own[id].(conversationPane)
	if !ok {
		return m.ShowError("/mode works in chat panes.")
	}
	pick := modepick.New(c.Mode())
	return m.showOverlay(&overlay{
		pane:  id,
		popup: pick,
		key:   pick.Key,
		click: pick.Click,
		done: func(r picker.Result) tea.Cmd {
			if mode := pick.Choice(); mode != "" && r == picker.Accepted {
				m.send(c.ModeInput(mode))
			}
			return nil
		},
	}, "the mode picker")
}

// helpView is the help panel as a popup.
type helpView struct{}

func (helpView) View(th theme.Theme, cols, rows int) (string, int, int) {
	return helppanel.Panel(th, cols, rows, command.Grouped(command.In(command.Chat)), chat.HelpKeys)
}

// openHelp shows the chat's commands and keys. Any key or click closes it.
func (m *Model) openHelp(id int) tea.Cmd {
	anything := func(tea.Key) picker.Result { return picker.Cancelled }
	return m.showOverlay(&overlay{
		pane:  id,
		popup: helpView{},
		key:   anything,
		click: func(theme.Theme, int, int, int, int) picker.Result { return picker.Cancelled },
	}, "the help panel")
}

// setConversationList gives the open resume picker the server's list.
func (m *Model) setConversationList(l *api.ConversationList) {
	if m.resume != nil {
		m.resume.SetList(l)
	}
}
