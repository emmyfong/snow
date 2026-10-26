package session

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/confirm"
	"github.com/emmyfong/snow/pkg/api"
)

// askLogout asks the server for the current key state, then confirms
// removing the key /logout would remove.
func (m *Model) askLogout(id int) tea.Cmd {
	if _, ok := m.own[id].(modelChooser); !ok {
		return m.ShowError("Logout works in chat panes.")
	}
	m.firstRun[id] = true
	delete(m.authWait, id)
	m.logoutPane = id
	m.send(&api.AuthListRequest{})
	return nil
}

// logoutDone says which saved key the server removed.
func (m *Model) logoutDone(l *api.Logout) tea.Cmd {
	name := l.Provider
	for _, p := range m.authList.Providers {
		if p.Name == l.Provider {
			if p.Title != "" {
				name = p.Title
			}
			break
		}
	}
	return m.showNotice("Removed the saved key for " + name + ".")
}

// finishLogout confirms removing the key of the pane's provider, or says
// why Snow cannot remove it.
func (m *Model) finishLogout() tea.Cmd {
	id := m.logoutPane
	m.logoutPane = 0
	p, ok := m.logoutProvider(id)
	if !ok {
		return m.ShowError("No provider has a saved key. Type /login to save one.")
	}
	title := p.Name
	if p.Title != "" {
		title = p.Title
	}
	if p.EnvVar != "" || p.Source == "environment" {
		env := p.EnvVar
		if env == "" {
			env = "the environment"
		}
		return m.ShowError(fmt.Sprintf("The key for %s is set in %s. Snow cannot remove it; unset it and restart snow.", title, env))
	}
	if p.Err != "" {
		return m.ShowError(fmt.Sprintf("Cannot read the key for %s: %s", title, p.Err))
	}
	m.confirm = &confirmation{
		popup: confirm.New("Log out", fmt.Sprintf("Remove the saved key for %s?", title), "Log out"),
		msg:   &api.LogoutRequest{Provider: p.Name},
	}
	return nil
}

// logoutProvider returns the key /logout would remove: the chat's provider
// when it has a key, else the first provider with one.
func (m *Model) logoutProvider(id int) (api.ProviderAuth, bool) {
	name := ""
	if c, ok := m.own[id].(modelChooser); ok {
		if model, _ := c.ModelName(); model != "" {
			name, _, _ = api.ParseModelName(model)
		}
	}
	if name != "" {
		for _, p := range m.authList.Providers {
			if p.Name == name && (p.Connected || p.Err != "") {
				return p, true
			}
		}
	}
	for _, p := range m.authList.Providers {
		if p.Connected {
			return p, true
		}
	}
	return api.ProviderAuth{}, false
}
