package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	sdk "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"

	"github.com/emmyfong/snow/internal/provider"
)

// streamChat sends req to the Chat Completions API. It sends no reasoning
// effort and keeps no reasoning blocks: services of this kind have no
// common way to take them.
func (p *Provider) streamChat(ctx context.Context, opts []option.RequestOption, key string, req provider.Request) (provider.Stream, error) {
	params, err := toChatParams(req)
	if err != nil {
		return nil, err
	}
	svc := sdk.NewChatCompletionService(opts...)
	return newChatStream(ctx, svc.NewStreaming(ctx, params), key), nil
}

// toChatParams turns a provider.Request into a Chat Completions request.
// It asks for usage in the stream, which many services leave out
// otherwise. max_tokens is sent, not max_completion_tokens: the services
// that use this API know the older name.
func toChatParams(req provider.Request) (sdk.ChatCompletionNewParams, error) {
	params := sdk.ChatCompletionNewParams{
		Model:         shared.ChatModel(req.Model),
		StreamOptions: sdk.ChatCompletionStreamOptionsParam{IncludeUsage: sdk.Bool(true)},
	}
	if req.MaxTokens > 0 {
		params.MaxTokens = sdk.Int(int64(req.MaxTokens))
	}
	if req.System != "" {
		params.Messages = append(params.Messages, sdk.SystemMessage(req.System))
	}
	for i, m := range req.Messages {
		msgs, err := toChatMessages(m)
		if err != nil {
			return sdk.ChatCompletionNewParams{}, fmt.Errorf("message %d: %w", i, err)
		}
		params.Messages = append(params.Messages, msgs...)
	}
	for _, t := range req.Tools {
		tool, err := toChatTool(t)
		if err != nil {
			return sdk.ChatCompletionNewParams{}, fmt.Errorf("tool %s: %w", t.Name, err)
		}
		params.Tools = append(params.Tools, tool)
	}
	return params, nil
}

// toChatMessages turns one message into chat messages. The API puts a tool
// result in its own message that must follow the assistant message with
// the call, so a user message gives its tool results first, then its text.
func toChatMessages(m provider.Message) ([]sdk.ChatCompletionMessageParamUnion, error) {
	var texts []string
	var results []sdk.ChatCompletionMessageParamUnion
	var calls []sdk.ChatCompletionMessageToolCallUnionParam
	for _, b := range m.Content {
		switch b := b.(type) {
		case *provider.Text:
			texts = append(texts, b.Text)
		case *provider.ToolUse:
			args := string(b.Input)
			if args == "" {
				args = "{}"
			}
			calls = append(calls, sdk.ChatCompletionMessageToolCallUnionParam{OfFunction: &sdk.ChatCompletionMessageFunctionToolCallParam{
				ID:       b.ID,
				Function: sdk.ChatCompletionMessageFunctionToolCallFunctionParam{Name: b.Name, Arguments: args},
			}})
		case *provider.ToolResult:
			out := b.Content
			if b.IsError {
				// The API has no error flag on a tool result.
				out = "Error: " + out
			}
			results = append(results, sdk.ToolMessage(out, b.ToolUseID))
		case *provider.Thinking, *provider.RedactedThinking:
			// Reasoning goes out only to the API that made it.
		default:
			return nil, fmt.Errorf("unknown block %T", b)
		}
	}
	text := strings.Join(texts, "\n\n")
	switch m.Role {
	case provider.RoleUser:
		out := results
		if len(texts) > 0 {
			out = append(out, sdk.UserMessage(text))
		}
		return out, nil
	case provider.RoleAssistant:
		if len(texts) == 0 && len(calls) == 0 {
			return nil, nil
		}
		msg := sdk.ChatCompletionAssistantMessageParam{ToolCalls: calls}
		if len(texts) > 0 {
			msg.Content.OfString = sdk.String(text)
		}
		return []sdk.ChatCompletionMessageParamUnion{{OfAssistant: &msg}}, nil
	}
	return nil, fmt.Errorf("unknown role %q", m.Role)
}

// toChatTool turns a tool definition into a function tool. strict is off:
// it needs every property required, which Snow's schemas do not promise.
func toChatTool(t provider.ToolDef) (sdk.ChatCompletionToolUnionParam, error) {
	schema := map[string]any{"type": "object", "properties": map[string]any{}}
	if len(t.Schema) > 0 {
		schema = map[string]any{}
		if err := json.Unmarshal(t.Schema, &schema); err != nil {
			return sdk.ChatCompletionToolUnionParam{}, fmt.Errorf("schema: %w", err)
		}
	}
	def := shared.FunctionDefinitionParam{Name: t.Name, Parameters: schema}
	if t.Description != "" {
		def.Description = sdk.String(t.Description)
	}
	return sdk.ChatCompletionFunctionTool(def), nil
}
