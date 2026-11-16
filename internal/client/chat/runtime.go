package chat

import (
	"fmt"
	"github.com/emmyfong/snow/internal/client/ui"
	"github.com/emmyfong/snow/pkg/api"
	"strings"
)

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

// SendNowInput takes the typed draft for an explicit interrupt-and-send action.
// Invalid input stays in the editor, just as with ordinary submission.
func (m *Model) SendNowInput() *api.ChatInput {
	text := string(m.input)
	if m.snap.State == api.ChatStateStopping {
		m.note = "Wait for the runtime to confirm its stop."
		return nil
	}
	if strings.TrimSpace(text) == "" || len(text) > api.MaxChatMessage {
		m.note = fmt.Sprintf("Type a message of at most %d KiB before choosing Interrupt and send.", api.MaxChatMessage>>10)
		return nil
	}
	m.input, m.cursor = nil, 0
	m.Follow()
	return m.chatInput(api.ChatInputSendNow, text)
}

func (m *Model) runtimeControlRows() []string {
	info := m.snap.Runtime
	route := "billing unavailable"
	if info.Auth == "subscription" {
		route = "subscription"
	}
	if info.Auth == "api-key" {
		route = "API key"
	}
	environment := ui.PlainLine(info.Environment)
	if environment == "" {
		environment = "environment unavailable"
	}
	label := fmt.Sprintf(" Run controls: %s, %s | %d tasks, %d drafts", route, environment, len(info.Tasks), len(m.snap.Queue))
	if m.snap.QueuePaused && len(m.snap.Queue) > 0 {
		label += " paused"
	}
	return []string{withHint(label, "Ctrl+T open", m.width)}
}
