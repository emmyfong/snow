package session

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/emmyfong/snow/internal/client/chat"
	"github.com/emmyfong/snow/internal/client/theme"
	"github.com/emmyfong/snow/pkg/api"
)

// chatPane fits a chat to ownPane.
type chatPane struct{ *chat.Model }

func newChatPane(id int, th theme.Theme, now func() time.Time, usage bool) chatPane {
	c := chat.Waiting(id, th, now)
	c.SetUsage(usage)
	return chatPane{c}
}

func (c chatPane) Feed(msg api.Message) {
	switch msg := msg.(type) {
	case *api.ChatSnapshot:
		c.Snapshot(msg)
	case *api.ChatEvent:
		c.Event(msg)
	}
}

func (c chatPane) Key(k tea.Key) (api.Message, string) { return c.result(c.Model.Key(k)) }

func (c chatPane) Click(x, y int) (api.Message, string) { return c.result(c.Model.Click(x, y)) }

func (c chatPane) Animating() bool { return c.Running() }

// result turns what the chat returned into a message, never a typed nil,
// and the note for what it asked the session to do.
func (c chatPane) result(in *api.ChatInput) (api.Message, string) {
	note := ""
	switch c.Asked() {
	case chat.AskLogin:
		// TODO(#114): open /login instead.
		note = "Run snow auth in a terminal to save a key, then retry."
	case chat.AskPickModel:
		// TODO(#113): open the model picker instead.
		note = "Set [chat] model in config.toml, then open a new chat."
	}
	if in == nil {
		return nil, note
	}
	return in, note
}
