package openai

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/emmyfong/snow/internal/provider"
)

// TestChatRequest: the request goes to /chat/completions with the system
// message, the history with a tool call and its result, the tool
// definitions, a usage request, and no reasoning effort.
func TestChatRequest(t *testing.T) {
	cs := newChatServer(t, "chat_text.sse")
	req := provider.Request{
		Model: "llama3.1:8b", Effort: provider.EffortHigh, System: "You are Snow.", MaxTokens: 512,
		Messages: []provider.Message{
			user("Read a.go"),
			{Role: provider.RoleAssistant, Content: []provider.Block{
				&provider.Thinking{Text: "hmm", Signature: "sig"},
				&provider.Text{Text: "Reading."},
				&provider.ToolUse{ID: "call_1", Name: "read_file", Input: json.RawMessage(`{"path":"a.go"}`)},
			}},
			{Role: provider.RoleUser, Content: []provider.Block{
				&provider.ToolResult{ToolUseID: "call_1", Content: "package a"},
				&provider.Text{Text: "Now summarize."},
			}},
		},
		Tools: []provider.ToolDef{{Name: "read_file", Description: "Read a file.", Schema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`)}},
	}
	if _, err := collectChat(t, chatProvider(cs.URL, false), req); err != nil {
		t.Fatal(err)
	}
	_, path, body := cs.last()
	if path != "/v1/chat/completions" {
		t.Fatalf("path %s", path)
	}
	if body["model"] != "llama3.1:8b" || body["stream"] != true || body["max_tokens"] != float64(512) {
		t.Fatalf("body %v", body)
	}
	for _, k := range []string{"reasoning_effort", "reasoning", "store"} {
		if _, has := body[k]; has {
			t.Errorf("body has %s: %v", k, body[k])
		}
	}
	if so, _ := body["stream_options"].(map[string]any); so["include_usage"] != true {
		t.Errorf("stream_options %v", body["stream_options"])
	}
	msgs, _ := body["messages"].([]any)
	var got []string
	for _, m := range msgs {
		mm := m.(map[string]any)
		got = append(got, fmt.Sprintf("%v|%v|%v", mm["role"], mm["content"], mm["tool_call_id"]))
	}
	want := []string{
		"system|You are Snow.|<nil>",
		"user|Read a.go|<nil>",
		"assistant|Reading.|<nil>",
		"tool|package a|call_1",
		"user|Now summarize.|<nil>",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("messages\n got %q\nwant %q", got, want)
	}
	calls, _ := msgs[2].(map[string]any)["tool_calls"].([]any)
	if len(calls) != 1 {
		t.Fatalf("tool_calls %v", calls)
	}
	call := calls[0].(map[string]any)
	fn := call["function"].(map[string]any)
	if call["id"] != "call_1" || call["type"] != "function" || fn["name"] != "read_file" || fn["arguments"] != `{"path":"a.go"}` {
		t.Fatalf("tool call %v", call)
	}
	tools, _ := body["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("tools %v", body["tools"])
	}
	def := tools[0].(map[string]any)["function"].(map[string]any)
	if tools[0].(map[string]any)["type"] != "function" || def["name"] != "read_file" || def["description"] != "Read a file." {
		t.Fatalf("tool %v", tools[0])
	}
}

// TestChatToolResultError: a failed tool result says so in its text, as the
// API has no error flag.
func TestChatToolResultError(t *testing.T) {
	cs := newChatServer(t, "chat_text.sse")
	req := provider.Request{Model: "m", Messages: []provider.Message{
		{Role: provider.RoleAssistant, Content: []provider.Block{&provider.ToolUse{ID: "c", Name: "run"}}},
		{Role: provider.RoleUser, Content: []provider.Block{&provider.ToolResult{ToolUseID: "c", Content: "boom", IsError: true}}},
	}}
	if _, err := collectChat(t, chatProvider(cs.URL, false), req); err != nil {
		t.Fatal(err)
	}
	_, _, body := cs.last()
	msgs, _ := body["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("messages %v", msgs)
	}
	if got := msgs[1].(map[string]any)["content"]; got != "Error: boom" {
		t.Fatalf("tool message %v", got)
	}
	// A tool call with no input still sends valid JSON arguments.
	calls, _ := msgs[0].(map[string]any)["tool_calls"].([]any)
	if args := calls[0].(map[string]any)["function"].(map[string]any)["arguments"]; args != "{}" {
		t.Fatalf("arguments %v", args)
	}
}

// TestChatMessageShapes: text blocks join into one message, an empty text
// block sends nothing, and a block in the wrong role is an error, not a
// silent drop.
func TestChatMessageShapes(t *testing.T) {
	textBlocks := func(texts ...string) []provider.Block {
		var bs []provider.Block
		for _, s := range texts {
			bs = append(bs, &provider.Text{Text: s})
		}
		return bs
	}
	roles := func(msgs ...provider.Message) []string {
		params, err := toChatParams(provider.Request{Model: "m", Messages: msgs})
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(params.Messages)
		if err != nil {
			t.Fatal(err)
		}
		var out []map[string]any
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, m := range out {
			got = append(got, fmt.Sprintf("%v|%v", m["role"], m["content"]))
		}
		return got
	}
	if got, want := roles(provider.Message{Role: provider.RoleUser, Content: textBlocks("one", "two")}), []string{"user|one\n\ntwo"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("joined %q, want %q", got, want)
	}
	if got := roles(provider.Message{Role: provider.RoleUser, Content: textBlocks("", "")}); len(got) != 0 {
		t.Fatalf("an empty text sent %q", got)
	}
	for _, tt := range []struct {
		name string
		msg  provider.Message
	}{
		{"tool use from the user", provider.Message{Role: provider.RoleUser, Content: []provider.Block{&provider.ToolUse{ID: "c", Name: "run"}}}},
		{"tool result from the assistant", provider.Message{Role: provider.RoleAssistant, Content: []provider.Block{&provider.ToolResult{ToolUseID: "c"}}}},
	} {
		if _, err := toChatParams(provider.Request{Model: "m", Messages: []provider.Message{tt.msg}}); err == nil {
			t.Errorf("%s: no error", tt.name)
		}
	}
}
