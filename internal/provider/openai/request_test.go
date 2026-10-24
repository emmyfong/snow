package openai

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/emmyfong/snow/internal/provider"
)

// TestRequestReasoning: reasoning and effort go only to a model that takes
// them; an unknown model gets the effort asked for.
func TestRequestReasoning(t *testing.T) {
	tests := []struct {
		model      string
		effort     provider.Effort
		reasoning  bool
		wantEffort string // "" means no effort field
	}{
		{"gpt-6.1-sol", provider.EffortMax, true, "max"},
		{"gpt-5.5-2026-04-23", provider.EffortMax, true, ""},
		{"gpt-5.5", provider.EffortXHigh, true, "xhigh"},
		{"gpt-6.1-sol", "", true, ""},
		{"gpt-4.1", provider.EffortHigh, false, ""},
		{"gpt-7", provider.EffortMax, true, "max"},
	}
	for _, tt := range tests {
		_, body, err := streamFixture(t, "text.sse", provider.Request{Model: tt.model, Effort: tt.effort, Messages: []provider.Message{user("Hi")}})
		if err != nil {
			t.Fatal(err)
		}
		r, hasReasoning := body["reasoning"].(map[string]any)
		if hasReasoning != tt.reasoning {
			t.Errorf("%s: reasoning %v, want %v", tt.model, body["reasoning"], tt.reasoning)
			continue
		}
		_, hasInclude := body["include"]
		if hasInclude != tt.reasoning {
			t.Errorf("%s: include %v", tt.model, body["include"])
		}
		if !tt.reasoning {
			continue
		}
		effort, _ := r["effort"].(string)
		if effort != tt.wantEffort || r["summary"] != "auto" {
			t.Errorf("%s %s: reasoning %v, want effort %q", tt.model, tt.effort, r, tt.wantEffort)
		}
	}
}

func TestRequestBasics(t *testing.T) {
	_, body, err := streamFixture(t, "text.sse", provider.Request{
		Model: "gpt-6.1-sol", System: "Be brief.", MaxTokens: 500, Messages: []provider.Message{user("Hi")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if body["model"] != "gpt-6.1-sol" || body["instructions"] != "Be brief." || body["max_output_tokens"] != 500.0 ||
		body["store"] != false || body["stream"] != true {
		t.Fatalf("body = %v", body)
	}
	if inc, _ := body["include"].([]any); len(inc) != 1 || inc[0] != "reasoning.encrypted_content" {
		t.Errorf("include = %v", body["include"])
	}
}

// TestRequestConversation: each block becomes its own input item, in
// order. Reasoning from OpenAI goes back unchanged; reasoning from another
// vendor is left out.
func TestRequestConversation(t *testing.T) {
	ref := reasoningRef{ID: "rs_1", Encrypted: "ENC-1"}.encode()
	hidden := reasoningRef{ID: "rs_2", Encrypted: "ENC-2"}.encode()
	msgs := []provider.Message{
		user("Read a.go"),
		{Role: provider.RoleAssistant, Content: []provider.Block{
			&provider.Thinking{Text: "Need the file.", Signature: ref},
			&provider.RedactedThinking{Data: hidden},
			&provider.Thinking{Text: "from Claude", Signature: "anthropic-signature"},
			&provider.RedactedThinking{Data: "anthropic-redacted"},
			&provider.Text{Text: "Reading it."},
			&provider.ToolUse{ID: "call_1", Name: "read", Input: json.RawMessage(`{"path":"a.go"}`)},
		}},
		{Role: provider.RoleUser, Content: []provider.Block{
			&provider.ToolResult{ToolUseID: "call_1", Content: "no such file", IsError: true},
			&provider.ToolResult{ToolUseID: "call_2", Content: "package main"},
		}},
	}
	_, body, err := streamFixture(t, "text.sse", provider.Request{Model: "gpt-6.1-sol", Messages: msgs})
	if err != nil {
		t.Fatal(err)
	}
	got := items(t, body)
	want := []map[string]any{
		{"role": "user", "content": "Read a.go"},
		{"type": "reasoning", "id": "rs_1", "encrypted_content": "ENC-1", "summary": []any{map[string]any{"type": "summary_text", "text": "Need the file."}}},
		{"type": "reasoning", "id": "rs_2", "encrypted_content": "ENC-2", "summary": []any{}},
		{"role": "assistant", "content": "Reading it."},
		{"type": "function_call", "call_id": "call_1", "name": "read", "arguments": `{"path":"a.go"}`},
		{"type": "function_call_output", "call_id": "call_1", "output": "Error: no such file"},
		{"type": "function_call_output", "call_id": "call_2", "output": "package main"},
	}
	if !reflect.DeepEqual(got, want) {
		gj, _ := json.MarshalIndent(got, "", " ")
		t.Fatalf("input items:\n%s", gj)
	}
}

// TestRequestNoReasoningModel: a model without reasoning gets no
// reasoning items, even its own from an earlier turn.
func TestRequestNoReasoningModel(t *testing.T) {
	ref := reasoningRef{ID: "rs_1", Encrypted: "ENC-1"}.encode()
	msgs := []provider.Message{user("Hi"), {Role: provider.RoleAssistant, Content: []provider.Block{
		&provider.Thinking{Text: "t", Signature: ref}, &provider.Text{Text: "Hello"},
	}}}
	_, body, err := streamFixture(t, "text.sse", provider.Request{Model: "gpt-4.1", Messages: msgs})
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items(t, body) {
		if it["type"] == "reasoning" {
			t.Fatalf("sent reasoning to gpt-4.1: %v", it)
		}
	}
}

func TestRequestTools(t *testing.T) {
	tools := []provider.ToolDef{
		{Name: "read", Description: "Read a file.", Schema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`)},
		{Name: "now"},
	}
	_, body, err := streamFixture(t, "text.sse", provider.Request{Model: "gpt-6.1-sol", Tools: tools, Messages: []provider.Message{user("Hi")}})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := body["tools"].([]any)
	want := []any{
		map[string]any{"type": "function", "name": "read", "description": "Read a file.", "strict": false,
			"parameters": map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}}, "required": []any{"path"}}},
		map[string]any{"type": "function", "name": "now", "strict": false,
			"parameters": map[string]any{"type": "object", "properties": map[string]any{}}},
	}
	if !reflect.DeepEqual(got, want) {
		gj, _ := json.MarshalIndent(got, "", " ")
		t.Fatalf("tools:\n%s", gj)
	}
}

func TestRequestBadInput(t *testing.T) {
	bad := []provider.Request{
		{Model: "m", Messages: []provider.Message{{Role: "system", Content: []provider.Block{&provider.Text{Text: "x"}}}}},
		{Model: "m", Tools: []provider.ToolDef{{Name: "t", Schema: json.RawMessage(`{`)}}},
	}
	for _, req := range bad {
		if _, err := toParams(req, capsFor(req.Model)); err == nil {
			t.Errorf("toParams(%+v) accepted bad input", req)
		}
	}
}
