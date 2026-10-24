package openai

import (
	"encoding/json"
	"fmt"

	sdk "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"

	"github.com/emmyfong/snow/internal/provider"
)

// toParams turns a provider.Request into the SDK's request for a model
// that takes mc. Nothing is stored on OpenAI's side (store: false), so
// reasoning comes back encrypted and goes out again with the next turn.
func toParams(req provider.Request, mc caps) (responses.ResponseNewParams, error) {
	params := responses.ResponseNewParams{
		Model: shared.ResponsesModel(req.Model),
		Store: sdk.Bool(false),
	}
	if req.MaxTokens > 0 {
		params.MaxOutputTokens = sdk.Int(int64(req.MaxTokens))
	}
	if req.System != "" {
		params.Instructions = sdk.String(req.System)
	}
	if mc.reasoning {
		params.Reasoning = shared.ReasoningParam{Summary: shared.ReasoningSummaryAuto}
		if req.Effort != "" && mc.takes(req.Effort) {
			params.Reasoning.Effort = shared.ReasoningEffort(req.Effort)
		}
		params.Include = []responses.ResponseIncludable{responses.ResponseIncludableReasoningEncryptedContent}
	}
	var input responses.ResponseInputParam
	for i, m := range req.Messages {
		items, err := toItems(m, mc.reasoning)
		if err != nil {
			return responses.ResponseNewParams{}, fmt.Errorf("message %d: %w", i, err)
		}
		input = append(input, items...)
	}
	params.Input = responses.ResponseNewParamsInputUnion{OfInputItemList: input}
	for _, t := range req.Tools {
		tool, err := toTool(t)
		if err != nil {
			return responses.ResponseNewParams{}, fmt.Errorf("tool %s: %w", t.Name, err)
		}
		params.Tools = append(params.Tools, tool)
	}
	return params, nil
}

// toItems turns one message into input items. The Responses API has no
// message with mixed parts: text, tool calls, tool results, and reasoning
// are each their own item, in the message's order.
func toItems(m provider.Message, reasoning bool) ([]responses.ResponseInputItemUnionParam, error) {
	var role responses.EasyInputMessageRole
	switch m.Role {
	case provider.RoleUser:
		role = responses.EasyInputMessageRoleUser
	case provider.RoleAssistant:
		role = responses.EasyInputMessageRoleAssistant
	default:
		return nil, fmt.Errorf("unknown role %q", m.Role)
	}
	var items []responses.ResponseInputItemUnionParam
	for _, b := range m.Content {
		switch b := b.(type) {
		case *provider.Text:
			items = append(items, responses.ResponseInputItemParamOfMessage(b.Text, role))
		case *provider.ToolUse:
			args := string(b.Input)
			if args == "" {
				args = "{}"
			}
			items = append(items, responses.ResponseInputItemParamOfFunctionCall(args, b.ID, b.Name))
		case *provider.ToolResult:
			out := b.Content
			if b.IsError {
				// The API has no error flag on a tool result.
				out = "Error: " + out
			}
			item := responses.ResponseInputItemParamOfFunctionCallOutput(out)
			item.OfFunctionCallOutput.CallID = sdk.String(b.ToolUseID)
			items = append(items, item)
		case *provider.Thinking:
			if item, ok := reasoningItem(b.Signature, b.Text, reasoning); ok {
				items = append(items, item)
			}
		case *provider.RedactedThinking:
			if item, ok := reasoningItem(b.Data, "", reasoning); ok {
				items = append(items, item)
			}
		default:
			return nil, fmt.Errorf("unknown block %T", b)
		}
	}
	return items, nil
}

// reasoningRef is what Snow keeps of an OpenAI reasoning item, in
// provider.Thinking's Signature or provider.RedactedThinking's Data. The
// agent loop keeps it opaque and sends it back unchanged.
type reasoningRef struct {
	ID        string `json:"openai_reasoning_id"`
	Encrypted string `json:"encrypted_content"`
}

func (r reasoningRef) encode() string {
	b, err := json.Marshal(r)
	if err != nil {
		// Two strings always marshal.
		panic(err)
	}
	return string(b)
}

// reasoningItem rebuilds a reasoning item from a kept reference. A block
// from another vendor, or one sent to a model without reasoning, is left
// out: the API accepts only its own reasoning, and only where it reasons.
func reasoningItem(ref, summary string, reasoning bool) (responses.ResponseInputItemUnionParam, bool) {
	var r reasoningRef
	if !reasoning || json.Unmarshal([]byte(ref), &r) != nil || r.ID == "" {
		return responses.ResponseInputItemUnionParam{}, false
	}
	parts := []responses.ResponseReasoningItemSummaryParam{}
	if summary != "" {
		parts = append(parts, responses.ResponseReasoningItemSummaryParam{Text: summary})
	}
	item := responses.ResponseInputItemParamOfReasoning(r.ID, parts)
	if r.Encrypted != "" {
		item.OfReasoning.EncryptedContent = sdk.String(r.Encrypted)
	}
	return item, true
}

// toTool turns a tool definition into a function tool. strict is off: it
// needs every property required, which Snow's schemas do not promise.
func toTool(t provider.ToolDef) (responses.ToolUnionParam, error) {
	schema := map[string]any{"type": "object", "properties": map[string]any{}}
	if len(t.Schema) > 0 {
		schema = map[string]any{}
		if err := json.Unmarshal(t.Schema, &schema); err != nil {
			return responses.ToolUnionParam{}, fmt.Errorf("schema: %w", err)
		}
	}
	tool := responses.ToolParamOfFunction(t.Name, schema, false)
	if t.Description != "" {
		tool.OfFunction.Description = sdk.String(t.Description)
	}
	return tool, nil
}
