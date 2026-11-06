package chat

import (
	"github.com/emmyfong/snow/internal/client/command"
	"github.com/emmyfong/snow/pkg/api"
)

// AskRuntime opens Snow-owned coding runtime connections.
const AskRuntime = "runtime"

// RuntimeInfo identifies the connection behind this pane, or nil for direct API.
func (m *Model) RuntimeInfo() *api.RuntimeInfo { return m.snap.Runtime }

// ConnectionRequest binds a connection operation to the current pane generation.
func (m *Model) ConnectionRequest(action, connection, auth, key string) *api.RuntimeRequest {
	return &api.RuntimeRequest{Pane: m.snap.Pane, Agent: m.snap.Agent, Action: action, Connection: connection, Auth: auth, Key: key}
}

// RuntimeInput builds an explicit run control with the current generation.
func (m *Model) RuntimeInput(kind, id, run, text string) *api.ChatInput {
	input := m.chatInput(kind, text)
	input.ID, input.Run = id, run
	return input
}

func init() {
	if err := Register(command.Command{Name: "connect", Desc: "Connect Claude Code to this chat", Group: command.GroupModel},
		func(host Host, _ string) { host.Ask(AskRuntime) }); err != nil {
		panic(err)
	}
}
