package session

import (
	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/modelpick"
	"github.com/emmyfong/snow/internal/client/picker"
	"github.com/emmyfong/snow/pkg/api"
)

// modelChooser is an own pane whose model and effort the model picker sets,
// such as a chat. The chat gives the session these two calls, and no more,
// so the session does not depend on the chat's model.
type modelChooser interface {
	// ModelName returns the pane's model as "provider/model", and its
	// effort. The model is empty until the user chooses one.
	ModelName() (model, effort string)
	// SwitchModelInput returns the input that makes the next turn use
	// model and effort. Save also makes it the default for new chats.
	SwitchModelInput(model, effort string, save bool) *api.ChatInput
}

// openModelPicker shows the model picker over pane id and asks the server
// for the provider lists.
func (m *Model) openModelPicker(id int) tea.Cmd {
	c, ok := m.own[id].(modelChooser)
	if !ok {
		return m.ShowError("The model picker works in chat panes.")
	}
	return m.showModelPicker(id, c)
}

// maybePickModel opens the model picker when a chat first reports no model,
// so the first message has a model to use. It opens once for each pane; a
// cancel leaves the chat empty, and its next message says why it did not
// send.
func (m *Model) maybePickModel(id int) tea.Cmd {
	if m.modelPick != nil || m.modelAsked[id] {
		return nil
	}
	p := m.focused()
	c, ok := m.own[id].(modelChooser)
	if p == nil || p.ID != id || !ok {
		return nil
	}
	if model, _ := c.ModelName(); model != "" {
		return nil
	}
	m.modelAsked[id] = true
	return m.showModelPicker(id, c)
}

// showModelPicker opens the model picker on pane id's model and effort.
func (m *Model) showModelPicker(id int, c modelChooser) tea.Cmd {
	model, effort := c.ModelName()
	pick := modelpick.New(model, effort)
	pick.SetList(m.modelList.Providers)
	if box, _, _ := pick.View(m.theme, m.cols, m.windowRows()); box == "" {
		return m.ShowError("Window too small for the model picker")
	}
	m.modelPick, m.modelPane = pick, id
	m.send(&api.ModelListRequest{})
	return nil
}

// modelKey sends a key to the open model picker.
func (m *Model) modelKey(k tea.Key) tea.Cmd {
	return m.modelResult(m.modelPick.Key(k))
}

// modelMouse handles a mouse event while the picker is open: a left click
// chooses or closes; a right click closes; the rest is ignored.
func (m *Model) modelMouse(msg tea.MouseMsg) tea.Cmd {
	click, ok := msg.(tea.MouseClickMsg)
	if !ok {
		return nil
	}
	if click.Button != tea.MouseLeft {
		m.modelPick = nil
		return nil
	}
	return m.modelResult(m.modelPick.Click(m.theme, m.cols, m.windowRows(), click.X, click.Y))
}

// modelResult applies what the picker chose to its chat and closes it. A
// saved choice also becomes the server's default for new chats.
func (m *Model) modelResult(r picker.Result) tea.Cmd {
	switch r {
	case picker.Accepted, picker.Saved:
		model, effort := m.modelPick.Choice()
		pane := m.own[m.modelPane]
		m.modelPick = nil
		if c, ok := pane.(modelChooser); ok && model != "" {
			if in := c.SwitchModelInput(model, effort, r == picker.Saved); in != nil {
				m.send(in)
			}
		}
	case picker.Cancelled:
		m.modelPick = nil
	}
	return nil
}

// overlayModelPick draws the open model picker over content, the window
// area of rows rows.
func (m *Model) overlayModelPick(content string, rows int) string {
	if m.modelPick == nil {
		return content
	}
	return m.overlayPopup(content, m.modelPick, rows)
}

// setModelList keeps the server's list for the next picker and updates the
// open one.
func (m *Model) setModelList(list *api.ModelList) {
	m.modelList = *list
	if m.modelPick != nil {
		m.modelPick.SetList(list.Providers)
	}
}
