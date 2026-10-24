package tool

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// fn is a tool made from a function.
type fn struct {
	def Def
	run func(context.Context, json.RawMessage) (string, error)
}

func (f fn) Def() Def { return f.def }

func (f fn) Run(ctx context.Context, _ Env, input json.RawMessage) (string, error) {
	return f.run(ctx, input)
}

func TestUnknownToolIsResult(t *testing.T) {
	got := NewRegistry().Call(context.Background(), Env{}, "nope", nil, nil)
	if !got.IsError || got.Content != `unknown tool "nope"` {
		t.Fatalf("got %+v", got)
	}
}

func TestToolErrorIsResult(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(fn{Def{Name: "fail"}, func(context.Context, json.RawMessage) (string, error) {
		return "", errors.New("disk full")
	}})
	if got := r.Call(context.Background(), Env{}, "fail", nil, nil); !got.IsError || got.Content != "disk full" {
		t.Fatalf("got %+v", got)
	}
}

// TestCallApproval: a denial skips Run and gives the reason; an empty
// input reaches the tool as {}.
func TestCallApproval(t *testing.T) {
	r := NewRegistry()
	var ran json.RawMessage
	_ = r.Register(fn{Def{Name: "edit", Changes: true}, func(_ context.Context, in json.RawMessage) (string, error) {
		ran = in
		return "ok", nil
	}})
	deny := func(context.Context, Def, json.RawMessage) (bool, string) { return false, "not now" }
	if got := r.Call(context.Background(), Env{}, "edit", nil, deny); !got.IsError || got.Content != "denied: not now" || ran != nil {
		t.Fatalf("denied call: %+v, ran %q", got, ran)
	}
	allow := func(_ context.Context, d Def, _ json.RawMessage) (bool, string) { return d.Changes, "" }
	if got := r.Call(context.Background(), Env{}, "edit", nil, allow); got.IsError || got.Content != "ok" || string(ran) != "{}" {
		t.Fatalf("allowed call: %+v, ran %q", got, ran)
	}
}
