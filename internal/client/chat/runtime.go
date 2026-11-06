package chat

import "github.com/emmyfong/snow/pkg/api"

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
