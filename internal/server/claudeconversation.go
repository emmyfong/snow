package server

import (
	"errors"

	"github.com/emmyfong/snow/internal/agent"
	"github.com/emmyfong/snow/internal/clauderuntime"
	"github.com/emmyfong/snow/pkg/api"
)

// claudeConversation keeps the server's legacy control types at its boundary.
// The runtime package has no dependency on Snow's direct-API agent loop.
type claudeConversation struct{ runtime *clauderuntime.Conversation }

func (c *claudeConversation) ID() string      { return c.runtime.ID() }
func (c *claudeConversation) Agent() agent.ID { return agent.ID(c.runtime.Agent()) }
func (*claudeConversation) Mode() agent.Mode  { return agent.ModeAsk }
func (c *claudeConversation) Idle() bool {
	state := c.runtime.Snapshot()
	return !state.Running && state.State != api.ChatStateStopping
}
func (c *claudeConversation) Send(text string) error {
	return c.input(api.ChatInput{Kind: api.ChatInputMessage, Text: text})
}
func (c *claudeConversation) Answer(answer agent.Answer) error {
	return c.input(api.ChatInput{Kind: api.ChatInputAnswer, ID: answer.ID, Decision: string(answer.Decision), Text: answer.Reason})
}
func (c *claudeConversation) Reply(reply agent.QuestionReply) error {
	return c.input(api.ChatInput{Kind: api.ChatInputReply, ID: reply.ID, Picked: reply.Picked, Text: reply.Other})
}
func (c *claudeConversation) Interrupt() error {
	return c.input(api.ChatInput{Kind: api.ChatInputInterrupt})
}
func (c *claudeConversation) Retry() error {
	return c.input(api.ChatInput{Kind: api.ChatInputRetry})
}
func (c *claudeConversation) SwitchModel(model, effort string) error {
	return c.input(api.ChatInput{Kind: api.ChatInputModel, Model: model, Effort: effort})
}
func (c *claudeConversation) Rename(title string) error {
	return c.input(api.ChatInput{Kind: api.ChatInputRename, Text: title})
}
func (c *claudeConversation) SetMode(mode agent.Mode) error {
	return c.input(api.ChatInput{Kind: api.ChatInputMode, Mode: string(mode)})
}
func (c *claudeConversation) Close() error { return c.runtime.Close() }

func (c *claudeConversation) input(input api.ChatInput) error {
	err := c.runtime.Input(input)
	if errors.Is(err, clauderuntime.ErrClosed) {
		return errors.Join(agent.ErrClosed, err)
	}
	return err
}
