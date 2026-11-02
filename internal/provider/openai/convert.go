package openai

import (
	"encoding/json"
	"fmt"

	"github.com/emmyfong/snow/internal/provider"
)

// The Responses and Chat Completions paths map Snow's requests and replies
// to two wire formats. What they map the same way is here.

// toolSchema returns the JSON schema of a tool's input as a map. A tool
// with no schema takes an object with no properties.
func toolSchema(t provider.ToolDef) (map[string]any, error) {
	if len(t.Schema) == 0 {
		return map[string]any{"type": "object", "properties": map[string]any{}}, nil
	}
	schema := map[string]any{}
	if err := json.Unmarshal(t.Schema, &schema); err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}
	return schema, nil
}

// toolOutput returns what a tool result says to the model. Neither API has
// an error flag on a tool result, so a failure says so in its text.
func toolOutput(r *provider.ToolResult) string {
	if r.IsError {
		return "Error: " + r.Content
	}
	return r.Content
}

// orEmptyObject returns arguments, or an empty JSON object when a call has
// none, because the APIs take no empty string for them.
func orEmptyObject(arguments string) string {
	if arguments == "" {
		return "{}"
	}
	return arguments
}

// usageOf converts an API's token counts. Its input total includes the
// cached and the cache-written tokens; provider.Usage keeps them apart.
func usageOf(input, output, cached, written int64) provider.Usage {
	return provider.Usage{
		Input:      max(int(input-cached-written), 0),
		Output:     int(output),
		CacheRead:  int(cached),
		CacheWrite: int(written),
	}
}
