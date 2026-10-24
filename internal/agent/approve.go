package agent

import (
	"context"
	"encoding/json"

	"github.com/emmyfong/snow/internal/tool"
)

// approve is the step before a tool runs.
func (c *Conversation) approve(context.Context, tool.Def, json.RawMessage) (bool, string) {
	return true, ""
}
