// Package runtimetest supplies model-free SDK protocol fixtures for subprocess tests.
package runtimetest

import (
	"encoding/json"
	"os"
	"strings"
)

// Peer runs only in a real helper subprocess. Failed assertions use
// exit codes rather than testing output, which would corrupt its NDJSON stream.
type Peer struct {
	decoder *json.Decoder
	encoder *json.Encoder
}

// NewPeer binds a protocol fixture to its helper subprocess streams.
func NewPeer() Peer {
	return Peer{decoder: json.NewDecoder(os.Stdin), encoder: json.NewEncoder(os.Stdout)}
}

// Read reads one message or ends the helper process on invalid input.
func (p Peer) Read() map[string]json.RawMessage {
	var message map[string]json.RawMessage
	if p.decoder.Decode(&message) != nil {
		os.Exit(2)
	}
	return message
}

// Write emits one NDJSON envelope.
func (p Peer) Write(message any) {
	if p.encoder.Encode(message) != nil {
		os.Exit(2)
	}
}

// Ack acknowledges a correlated control request. The reply carries the
// model list Claude Code's initialize reply has.
func (p Peer) Ack(message map[string]json.RawMessage) {
	p.Write(map[string]any{"type": "control_response", "response": map[string]any{
		"subtype": "success", "request_id": message["request_id"], "response": map[string]any{"commands": []string{},
			"models": Models}}})
}

// Models is the model list a fixture's initialize reply carries.
var Models = []map[string]any{
	{"value": "default", "displayName": "Default (recommended)", "supportedEffortLevels": []string{"low", "medium", "high", "xhigh", "max"}},
	{"value": "opus", "displayName": "Opus", "supportedEffortLevels": []string{"low", "medium", "high", "xhigh", "max"}},
	{"value": "haiku", "displayName": "Haiku"},
}

// Permission emits a tool decision request.
func (p Peer) Permission(id, tool string, input any) {
	p.Write(map[string]any{"type": "control_request", "request_id": id, "request": map[string]any{
		"subtype": "can_use_tool", "tool_name": tool, "tool_use_id": "tool-" + id, "input": input}})
}

// Conversation serves a deterministic SDK-shaped conversation in a helper process.
// It does not contact a model or represent live account verification.
func Conversation(mode string) {
	p := NewPeer()
	p.Ack(p.Read())
	turn := 0
	for {
		message := p.Read()
		switch string(message["type"]) {
		case `"control_request"`:
			var request map[string]string
			if json.Unmarshal(message["request"], &request) != nil {
				os.Exit(3)
			}
			p.Ack(message)
			if request["subtype"] == "stop_task" && mode != "conversation-children-unconfirmed" {
				p.Write(map[string]any{"type": "system", "subtype": "task_updated", "task_id": request["task_id"],
					"run_id": "run-1", "patch": map[string]string{"status": "killed"}})
			}
		case `"user"`:
			turn++
			p.Write(map[string]string{"type": "system", "subtype": "init", "session_id": "backend-session", "model": "test-model"})
			p.Write(map[string]string{"type": "system", "subtype": "session_state_changed", "state": "running"})
			if mode == "conversation-stop" {
				p.Permission("approval", "Bash", map[string]string{"command": "echo test"})
				continue
			}
			if strings.HasPrefix(mode, "conversation-children") {
				for _, task := range []struct{ id, parent string }{{"child", ""}, {"grandchild", "child"}, {"sibling", ""}} {
					p.Write(map[string]any{"type": "system", "subtype": "task_started", "task_id": task.id,
						"parent_task_id": task.parent, "run_id": "run-1", "task_type": "local_agent", "description": task.id})
				}
				continue
			}
			if turn == 1 && mode != "conversation-fast" {
				conversationQuestions(p)
			}
			text, chunks := "hello world", 1
			if mode == "conversation-fast" {
				text, chunks = "x", 300
			}
			for range chunks {
				p.Write(map[string]any{"type": "stream_event", "event": map[string]any{"type": "content_block_delta",
					"delta": map[string]string{"type": "text_delta", "text": text}}})
			}
			p.Write(map[string]any{"type": "assistant", "message": map[string]any{"content": []map[string]string{{"type": "text", "text": strings.Repeat(text, chunks)}}}})
			p.Write(map[string]any{"type": "result", "session_id": "backend-session", "subtype": "success",
				"total_cost_usd": float64(turn) / 10, "usage": map[string]int{"input_tokens": 10, "output_tokens": 5, "cache_read_input_tokens": 8}})
			p.Write(map[string]string{"type": "system", "subtype": "session_state_changed", "state": "idle"})
		}
	}
}

func conversationQuestions(p Peer) {
	p.Permission("questions", "AskUserQuestion", map[string]any{"questions": []map[string]any{
		{"question": "Pick features", "multiSelect": true, "options": []map[string]string{{"label": "A"}, {"label": "B"}}},
		{"question": "Add a note", "options": []map[string]string{{"label": "Default"}}},
	}})
	response := p.Read()
	var answer struct {
		RequestID string `json:"request_id"`
		Response  struct {
			Behavior string `json:"behavior"`
			Input    struct {
				Answers map[string]string `json:"answers"`
			} `json:"updatedInput"`
		} `json:"response"`
	}
	if json.Unmarshal(response["response"], &answer) != nil || answer.RequestID != "questions" || answer.Response.Behavior != "allow" ||
		answer.Response.Input.Answers["Pick features"] != "A, B" || answer.Response.Input.Answers["Add a note"] != "raw\nfree answer" {
		os.Exit(4)
	}
	p.Permission("approval", "Bash", map[string]string{"command": "echo test"})
	response = p.Read()
	if json.Unmarshal(response["response"], &answer) != nil || answer.RequestID != "approval" || answer.Response.Behavior != "allow" {
		os.Exit(5)
	}
}
