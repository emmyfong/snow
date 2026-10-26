package session

import (
	"maps"
	"slices"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/authpick"
	"github.com/emmyfong/snow/pkg/api"
)

// openLoginPicker shows the login picker over pane id and asks the server
// for a fresh provider list.
func (m *Model) openLoginPicker(id int) tea.Cmd {
	if _, ok := m.own[id].(modelChooser); !ok {
		return m.ShowError("The login picker works in chat panes.")
	}
	if m.loginPane != 0 {
		return m.ShowError("A key check is still running.")
	}
	if m.authPick != nil {
		return nil
	}
	pick := authpick.New()
	pick.SetList(m.authList.Providers)
	if box, _, _ := pick.View(m.theme, m.cols, m.windowRows()); box == "" {
		return m.ShowError("Window too small for the login picker")
	}
	m.authPick, m.authPane = pick, id
	m.firstRun[id] = true
	delete(m.authWait, id)
	m.send(&api.AuthListRequest{})
	return nil
}

// anyConnected reports whether any provider in list has a key.
func anyConnected(list api.AuthList) bool {
	for _, p := range list.Providers {
		if p.Connected {
			return true
		}
	}
	return false
}

// authKey sends a key to the open login picker.
func (m *Model) authKey(k tea.Key) tea.Cmd { return m.authResult(m.authPick.Key(k)) }

// authMouse handles a mouse event while the login picker is open: a left
// click chooses a provider or closes outside; a right click closes.
func (m *Model) authMouse(msg tea.MouseMsg) tea.Cmd {
	click, ok := msg.(tea.MouseClickMsg)
	if !ok {
		return nil
	}
	if click.Button != tea.MouseLeft {
		return m.authClose()
	}
	return m.authResult(m.authPick.Click(m.theme, m.cols, m.windowRows(), click.X, click.Y))
}

// authResult applies what the login picker chose.
func (m *Model) authResult(r authpick.Result) tea.Cmd {
	switch r {
	case authpick.Submit:
		p, ok := m.authPick.Provider()
		if !ok {
			return nil
		}
		m.loginPane = m.authPane
		m.send(&api.LoginRequest{Provider: p.Name, Key: m.authPick.KeyValue()})
	case authpick.Cancelled:
		return m.authClose()
	}
	return nil
}

// authClose closes the login picker and, after a saved key, opens the
// model picker when the chat still has no model. The saved key is enough:
// the fresh key list may not have reached the client yet.
func (m *Model) authClose() tea.Cmd {
	pane := m.authPane
	m.authPick, m.authPane = nil, 0
	if !m.authAfter[pane] {
		return nil
	}
	c, ok := m.own[pane].(modelChooser)
	if !ok || m.modelPick != nil {
		delete(m.authAfter, pane)
		return nil
	}
	if model, _ := c.ModelName(); model != "" {
		delete(m.authAfter, pane)
		return nil
	}
	if p := m.focused(); p == nil || p.ID != pane {
		return nil
	}
	delete(m.authAfter, pane)
	return m.showModelPicker(pane, c)
}

// setAuthList keeps the server's key state, tells every chat whether a key
// is ready, and continues a first-run or /logout flow that waited for it.
func (m *Model) setAuthList(list *api.AuthList) tea.Cmd {
	m.authList = *list
	connected := anyConnected(m.authList)
	for _, p := range m.own {
		if r, ok := p.(providerReady); ok {
			r.SetProviderReady(connected)
		}
	}
	if m.authPick != nil {
		m.authPick.SetList(list.Providers)
	}
	if m.logoutPane != 0 {
		return m.finishLogout()
	}
	if m.authPick != nil || m.modelPick != nil {
		clear(m.authWait)
		return nil
	}
	if cmd := m.openFirstChat(connected, len(list.Providers) == 0); cmd != nil {
		return cmd
	}
	return m.openModelAfterLogin(connected)
}

// openFirstChat opens what the first chat of a pane needs: the model picker
// when a provider has a key, else the login picker.
func (m *Model) openFirstChat(connected, noProviders bool) tea.Cmd {
	if len(m.authWait) == 0 {
		return nil
	}
	id := m.firstWaitingPane()
	clear(m.authWait)
	c, ok := m.own[id].(modelChooser)
	if !ok {
		return nil
	}
	if model, _ := c.ModelName(); model != "" {
		return nil
	}
	if p := m.focused(); p == nil || p.ID != id {
		return nil
	}
	if connected || noProviders {
		return m.showModelPicker(id, c)
	}
	return m.openLoginPicker(id)
}

// firstWaitingPane returns the waiting pane that has the focus, else the
// first one by ID.
func (m *Model) firstWaitingPane() int {
	ids := slices.Sorted(maps.Keys(m.authWait))
	if len(ids) == 0 {
		return 0
	}
	if p := m.focused(); p != nil && m.authWait[p.ID] {
		return p.ID
	}
	return ids[0]
}

// openModelAfterLogin opens the model picker once a saved key has closed
// the login picker and the chat still has no model.
func (m *Model) openModelAfterLogin(connected bool) tea.Cmd {
	if len(m.authAfter) == 0 || !connected {
		return nil
	}
	for _, id := range slices.Sorted(maps.Keys(m.authAfter)) {
		delete(m.authAfter, id)
		c, ok := m.own[id].(modelChooser)
		if !ok {
			continue
		}
		if model, _ := c.ModelName(); model != "" {
			continue
		}
		if p := m.focused(); p == nil || p.ID != id {
			continue
		}
		return m.showModelPicker(id, c)
	}
	return nil
}

// loginDone puts a successful login in the picker, or in the status bar
// when the picker closed while the check ran.
func (m *Model) loginDone(l *api.Login) tea.Cmd {
	pane := m.loginPane
	m.loginPane = 0
	if pane == 0 {
		pane = m.authPane
	}
	if pane != 0 {
		m.authAfter[pane] = true
	}
	if m.authPick != nil {
		m.authPick.Saved(l)
		return nil
	}
	return m.showNotice(authpick.SavedText(l))
}

// authError sends a failed login check to the picker, or shows it when the
// picker closed while the check ran. It reports false for other failures.
func (m *Model) authError(text string) (tea.Cmd, bool) {
	if m.authPick != nil && m.authPick.Checking() {
		m.loginPane = 0
		m.authPick.Fail(text)
		return nil, true
	}
	if m.loginPane != 0 {
		m.loginPane = 0
		clear(m.authWait)
		return m.ShowError(text), true
	}
	return nil, false
}

// overlayAuthPick draws the open login picker over content, the window area
// of rows rows.
func (m *Model) overlayAuthPick(content string, rows int) string {
	if m.authPick == nil {
		return content
	}
	return m.overlayPopup(content, m.authPick, rows)
}
