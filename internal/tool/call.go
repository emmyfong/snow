package tool

import (
	"context"
	"encoding/json"
	"fmt"
)

// Result is what a tool call gives back to the model.
type Result struct {
	Content string
	IsError bool
}

// Approver is the step before a tool runs. It returns whether the call may
// run and, when it may not, the reason that goes back to the model. It may
// wait, for example for the user, and must stop when ctx ends.
type Approver func(ctx context.Context, def Def, input json.RawMessage) (allowed bool, reason string)

// Call runs the tool called name. An unknown name, a denial, and a tool
// error all come back as a failed Result for the model, never as a Go
// error: the model can read them and try again.
func (r *Registry) Call(ctx context.Context, env Env, name string, input json.RawMessage, approve Approver) Result {
	t, ok := r.Get(name)
	if !ok {
		return Result{Content: fmt.Sprintf("unknown tool %q", name), IsError: true}
	}
	if len(input) == 0 {
		input = json.RawMessage("{}")
	}
	if approve != nil {
		if allowed, reason := approve(ctx, t.Def(), input); !allowed {
			return Result{Content: "denied: " + reason, IsError: true}
		}
	}
	out, err := t.Run(ctx, env, input)
	if err != nil {
		return Result{Content: err.Error(), IsError: true}
	}
	return Result{Content: out}
}
