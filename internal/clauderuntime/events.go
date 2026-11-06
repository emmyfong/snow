package clauderuntime

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/emmyfong/snow/pkg/api"
)

type contentBlock struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Text      string          `json:"text"`
	Thinking  string          `json:"thinking"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
}

type runtimeMessage struct {
	Type            string  `json:"type"`
	Subtype         string  `json:"subtype"`
	SessionID       string  `json:"session_id"`
	ParentToolUseID *string `json:"parent_tool_use_id"`
	Model           string  `json:"model"`
	TaskID          string  `json:"task_id"`
	RunID           string  `json:"run_id"`
	ParentTaskID    string  `json:"parent_task_id"`
	TaskType        string  `json:"task_type"`
	Description     string  `json:"description"`
	Status          string  `json:"status"`
	State           string  `json:"state"`
	Ambient         bool    `json:"ambient"`
	SpawnDepth      int     `json:"spawn_depth"`
	Patch           struct {
		Status string `json:"status"`
	} `json:"patch"`
	Message struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
	Event struct {
		Type  string `json:"type"`
		Delta struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			Thinking string `json:"thinking"`
		} `json:"delta"`
	} `json:"event"`
	IsError bool     `json:"is_error"`
	Result  string   `json:"result"`
	Cost    *float64 `json:"total_cost_usd"`
	Usage   struct {
		Input      int `json:"input_tokens"`
		Output     int `json:"output_tokens"`
		CacheRead  int `json:"cache_read_input_tokens"`
		CacheWrite int `json:"cache_creation_input_tokens"`
	} `json:"usage"`
}

func (s *conversationState) frame(frame Frame) error {
	if frame.Type == "control_request" {
		if s.stopped != nil {
			return nil
		}
		return s.permission(frame)
	}
	if frame.Type == "control_cancel_request" {
		s.cancelPermission(frame.RequestID)
		return nil
	}
	var message runtimeMessage
	if json.Unmarshal(frame.Raw, &message) != nil {
		return fmt.Errorf("%w: invalid event payload", ErrProtocol)
	}
	root := message.ParentToolUseID == nil
	if root && (message.Type == "result" || message.Type == "system" && message.Subtype == "init") && message.SessionID != "" {
		if s.snapshot.Runtime.Session != "" && s.snapshot.Runtime.Session != message.SessionID {
			return fmt.Errorf("%w: runtime changed session identity", ErrProtocol)
		}
		info := *s.snapshot.Runtime
		info.Session = message.SessionID
		s.emit(api.ChatEvent{Kind: api.ChatEventRuntime, Runtime: &info})
	}
	switch message.Type {
	case "system":
		switch message.Subtype {
		case "init":
			// The model Claude reports here is what it resolved the chat's
			// choice to. The chat keeps its choice, so a later start still
			// follows Claude's default, and its effort is kept.
		case "session_state_changed":
			s.sessionState = message.State
			if message.State != "idle" {
				s.reopen()
			}
			s.maybeComplete()
		case "task_started", "task_updated", "task_notification":
			return s.taskEvent(message)
		}
	case "stream_event":
		if root {
			s.delta(message)
		}
	case "assistant":
		return s.assistant(message, root)
	case "user":
		var blocks []contentBlock
		if json.Unmarshal(message.Message.Content, &blocks) == nil {
			for _, block := range blocks {
				if block.Type == "tool_result" {
					s.emit(api.ChatEvent{Kind: api.ChatEventToolResult, Tool: &api.ChatTool{
						ID: block.ToolUseID, Done: true, Result: toolResultText(block.Content), IsError: block.IsError}})
				}
			}
		}
	case "result":
		if root {
			s.result(message)
		}
	}
	return nil
}

func (s *conversationState) delta(message runtimeMessage) {
	delta := message.Event.Delta
	switch message.Event.Type {
	case "message_start":
		s.reopen()
		s.streamed, s.streamedThinking, s.resultSeen = false, false, false
	case "content_block_delta":
		switch delta.Type {
		case "text_delta":
			s.streamed, s.sawText = true, true
			s.emit(api.ChatEvent{Kind: api.ChatEventText, Text: delta.Text})
		case "thinking_delta":
			s.streamedThinking = true
			s.emit(api.ChatEvent{Kind: api.ChatEventThinking, Text: delta.Thinking})
		}
	}
}

func (s *conversationState) assistant(message runtimeMessage, root bool) error {
	if root {
		s.reopen()
	}
	var blocks []contentBlock
	if json.Unmarshal(message.Message.Content, &blocks) != nil {
		return fmt.Errorf("%w: invalid assistant blocks", ErrProtocol)
	}
	for _, block := range blocks {
		switch block.Type {
		case "text":
			if root && !s.streamed {
				s.sawText = true
				s.emit(api.ChatEvent{Kind: api.ChatEventText, Text: block.Text})
			}
		case "thinking":
			if root && !s.streamedThinking {
				s.emit(api.ChatEvent{Kind: api.ChatEventThinking, Text: block.Thinking})
			}
		case "tool_use":
			s.emit(api.ChatEvent{Kind: api.ChatEventToolCall, Tool: &api.ChatTool{ID: block.ID, Name: block.Name, Input: block.Input}})
		}
	}
	if root {
		s.streamed, s.streamedThinking = false, false
	}
	return nil
}

func (s *conversationState) reopen() {
	if s.stopped == nil && !s.snapshot.Running && s.snapshot.Turn > 0 {
		s.emit(api.ChatEvent{Kind: api.ChatEventTurnStarted, Turn: s.snapshot.Turn})
		s.emit(api.ChatEvent{Kind: api.ChatEventState, State: api.ChatStateStreaming})
	}
}

func toolResultText(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var blocks []contentBlock
	if json.Unmarshal(raw, &blocks) == nil {
		var parts []string
		for _, block := range blocks {
			if block.Type == "text" {
				parts = append(parts, block.Text)
			}
		}
		return strings.Join(parts, "\n")
	}
	return "Tool returned structured content."
}
