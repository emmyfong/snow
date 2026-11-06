package server

import "github.com/emmyfong/snow/internal/agent"

// conversation is the server's chat control boundary. The direct-API agent
// implements it itself; an external runtime adapter translates these controls
// without pretending to be a provider or importing the server.
type conversation interface {
	ID() string
	Agent() agent.ID
	Mode() agent.Mode
	Idle() bool
	Send(string) error
	Answer(agent.Answer) error
	Reply(agent.QuestionReply) error
	Interrupt() error
	Retry() error
	SwitchModel(string, string) error
	Rename(string) error
	SetMode(agent.Mode) error
	Close() error
}
