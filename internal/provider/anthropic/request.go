package anthropic

import (
	"encoding/json"
	"fmt"

	sdk "github.com/anthropics/anthropic-sdk-go"

	"github.com/emmyfong/snow/internal/provider"
)

// toParams turns a provider.Request into the SDK's request. Thinking is
// adaptive with summaries, so the chat can show the model's reasoning.
func toParams(req provider.Request) (sdk.MessageNewParams, error) {
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = defaultMaxTokens
	}
	adaptive := sdk.ThinkingConfigAdaptiveParam{Display: sdk.ThinkingConfigAdaptiveDisplaySummarized}
	params := sdk.MessageNewParams{
		Model:     sdk.Model(req.Model),
		MaxTokens: int64(maxTokens),
		Thinking:  sdk.ThinkingConfigParamUnion{OfAdaptive: &adaptive},
	}
	if req.Effort != "" {
		params.OutputConfig = sdk.OutputConfigParam{Effort: sdk.OutputConfigEffort(req.Effort)}
	}
	if req.System != "" {
		params.System = []sdk.TextBlockParam{{Text: req.System}}
	}
	for i, m := range req.Messages {
		msg, err := toMessage(m)
		if err != nil {
			return sdk.MessageNewParams{}, fmt.Errorf("message %d: %w", i, err)
		}
		params.Messages = append(params.Messages, msg)
	}
	for _, t := range req.Tools {
		tool, err := toTool(t)
		if err != nil {
			return sdk.MessageNewParams{}, fmt.Errorf("tool %s: %w", t.Name, err)
		}
		params.Tools = append(params.Tools, tool)
	}
	return params, nil
}

func toMessage(m provider.Message) (sdk.MessageParam, error) {
	blocks := make([]sdk.ContentBlockParamUnion, 0, len(m.Content))
	for _, b := range m.Content {
		switch b := b.(type) {
		case *provider.Text:
			blocks = append(blocks, sdk.NewTextBlock(b.Text))
		case *provider.Thinking:
			blocks = append(blocks, sdk.NewThinkingBlock(b.Signature, b.Text))
		case *provider.ToolUse:
			blocks = append(blocks, sdk.NewToolUseBlock(b.ID, b.Input, b.Name))
		case *provider.ToolResult:
			blocks = append(blocks, sdk.NewToolResultBlock(b.ToolUseID, b.Content, b.IsError))
		default:
			return sdk.MessageParam{}, fmt.Errorf("unknown block %T", b)
		}
	}
	switch m.Role {
	case provider.RoleUser:
		return sdk.NewUserMessage(blocks...), nil
	case provider.RoleAssistant:
		return sdk.NewAssistantMessage(blocks...), nil
	default:
		return sdk.MessageParam{}, fmt.Errorf("unknown role %q", m.Role)
	}
}

// toTool splits a JSON Schema into the SDK's fields: properties, required,
// and everything else as extra fields.
func toTool(t provider.ToolDef) (sdk.ToolUnionParam, error) {
	schema := map[string]any{}
	if len(t.Schema) > 0 {
		if err := json.Unmarshal(t.Schema, &schema); err != nil {
			return sdk.ToolUnionParam{}, fmt.Errorf("schema: %w", err)
		}
	}
	in := sdk.ToolInputSchemaParam{Properties: schema["properties"], ExtraFields: map[string]any{}}
	if req, ok := schema["required"].([]any); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				in.Required = append(in.Required, s)
			}
		}
	}
	for k, v := range schema {
		if k != "properties" && k != "required" && k != "type" {
			in.ExtraFields[k] = v
		}
	}
	tool := sdk.ToolParam{Name: t.Name, InputSchema: in}
	if t.Description != "" {
		tool.Description = sdk.String(t.Description)
	}
	return sdk.ToolUnionParam{OfTool: &tool}, nil
}
