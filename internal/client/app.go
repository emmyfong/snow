package client

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/session"
	"github.com/emmyfong/snow/pkg/api"
)

// Result says how a client run ended.
type Result struct {
	Session  string // the attached session's name
	Detached bool   // the user detached; the session keeps running
	Err      error  // the server refused the request, for example *api.Error
}

// connClosedMsg reports that the server closed the connection.
type connClosedMsg struct{}

// app is the root model. It routes messages to the current screen and ends
// the program on detach, on a server error, or when the connection closes.
type app struct {
	screen Screen
	send   func(api.Message)
	result Result
}

func (a *app) Init() tea.Cmd { return nil }

func (a *app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.screen.SetSize(msg.Width, msg.Height)
		a.send(&api.Resize{Cols: msg.Width, Rows: msg.Height})
		return a, nil
	case *api.Layout:
		a.result.Session = msg.Session
	case *api.Error:
		a.result.Err = msg
		return a, tea.Quit
	case connClosedMsg:
		return a, tea.Quit
	case session.DetachMsg:
		a.result.Detached = true
		return a, tea.Quit
	}
	return a, a.screen.Update(msg)
}

func (a *app) View() tea.View {
	v := a.screen.View()
	v.AltScreen = true
	return v
}
