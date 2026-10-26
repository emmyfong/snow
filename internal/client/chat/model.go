package chat

import (
	"strings"

	"github.com/emmyfong/snow/internal/client/command"
	"github.com/emmyfong/snow/pkg/api"
)

// ModelName returns the model the chat uses, as "provider/model", and its
// effort. The model is empty until the user chooses one.
func (m *Model) ModelName() (model, effort string) { return m.snap.Model, m.snap.Effort }

// SwitchModelInput returns the input that makes the next turn use model
// with effort. Save also makes it the default for new chats.
func (m *Model) SwitchModelInput(model, effort string, save bool) *api.ChatInput {
	in := m.chatInput(api.ChatInputModel, "")
	in.Model, in.Effort, in.SaveDefault = model, effort, save
	return in
}

// runModel handles /model: with no argument it asks the session for the
// picker; with a model it switches to it. A model without a provider uses
// the chat's provider.
func runModel(h Host, rest string) {
	fields := strings.Fields(rest)
	switch {
	case len(fields) == 0:
		h.Ask(AskPickModel)
		return
	case len(fields) > 1:
		h.Notice("Type /model provider/model, or /model to pick one.")
		return
	}
	name := fields[0]
	current, effort := h.Model()
	if !strings.Contains(name, "/") {
		if provider, _, ok := api.ParseModelName(current); ok {
			name = provider + "/" + name
		}
	}
	if _, _, ok := api.ParseModelName(name); !ok {
		h.Notice("Type /model provider/model, or /model to pick one.")
		return
	}
	h.SwitchModel(name, effort)
}

func init() {
	if err := Register(command.Command{
		Name: "model", Args: "[id]",
		Desc:  "Pick a model, or switch to one",
		Group: command.GroupModel,
	}, runModel); err != nil {
		panic(err)
	}
}
