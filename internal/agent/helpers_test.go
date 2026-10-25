package agent

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/convstore"
	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providertest"
	"github.com/emmyfong/snow/internal/tool"
)

// testModel is the model of test conversations.
const testModel = "fake/model-1"

// harness is a conversation driven by a fake provider in a test.
type harness struct {
	conv   *Conversation
	fake   *providertest.Fake
	store  *convstore.Store
	tools  *tool.Registry
	folder string
	events chan Event
}

// newHarness starts a conversation whose provider plays replies. Tools
// are added to an empty registry.
func newHarness(t *testing.T, tools []tool.Tool, replies ...providertest.Reply) *harness {
	t.Helper()
	store, err := convstore.Open(filepath.Join(t.TempDir(), "conversations"))
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{fake: providertest.New("fake", replies...), store: store, tools: tool.NewRegistry(), folder: t.TempDir(), events: make(chan Event, 1000)}
	for _, tl := range tools {
		if err := h.tools.Register(tl); err != nil {
			t.Fatal(err)
		}
	}
	h.conv, err = Start(h.config(t), convstore.Meta{Model: testModel})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.conv.Close() })
	return h
}

func (h *harness) config(t *testing.T) Config {
	providers := provider.NewRegistry()
	if err := providers.Register(h.fake); err != nil {
		t.Fatal(err)
	}
	return Config{Store: h.store, Providers: providers, Tools: h.tools,
		Folder: tool.Folder{Path: h.folder}, Events: func(e Event) { h.events <- e }}
}

// next waits for the next event whose body is a T, and returns it. Other
// events are skipped.
func next[T Body](t *testing.T, h *harness) T {
	t.Helper()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case e := <-h.events:
			if b, ok := e.Body.(T); ok {
				return b
			}
		case <-timeout:
			var zero T
			t.Fatalf("no %T event", zero)
			return zero
		}
	}
}

// records loads the conversation file, without its meta record.
func (h *harness) records(t *testing.T) []convstore.Record {
	t.Helper()
	c, err := h.store.Load(h.conv.ID())
	if err != nil {
		t.Fatal(err)
	}
	return c.Records
}

func types(rs []convstore.Record) []convstore.Type {
	var ts []convstore.Type
	for _, r := range rs {
		ts = append(ts, r.Type())
	}
	return ts
}

// text is a reply that says s and ends the turn.
func text(s string) providertest.Reply {
	return providertest.Reply{Events: []provider.Event{
		&provider.Start{Model: "model-1-20261001"}, &provider.TextDelta{Text: s},
		&provider.Usage{Input: 10, Output: 2}, &provider.Stop{Reason: provider.StopEndTurn},
	}}
}

// call is a reply that calls the tool name once with id.
func call(id, name string) providertest.Reply {
	return providertest.Reply{Events: []provider.Event{
		&provider.TextDelta{Text: "Let me check."},
		&provider.ToolUse{ID: id, Name: name, Input: json.RawMessage(`{}`)},
		&provider.Stop{Reason: provider.StopToolUse},
	}}
}

// fnTool is a tool made from a function.
type fnTool struct {
	def tool.Def
	run func(context.Context) (string, error)
}

func (f fnTool) Def() tool.Def { return f.def }

func (f fnTool) Run(ctx context.Context, _ tool.Env, _ json.RawMessage) (string, error) {
	return f.run(ctx)
}

// echo is a tool that answers "echoed".
var echo = fnTool{tool.Def{Name: "echo", Schema: json.RawMessage(`{"type":"object"}`)},
	func(context.Context) (string, error) { return "echoed", nil }}

// gate is a tool that runs until release is closed or its context ends,
// and reports its context's error on ctxErr.
func gate(release <-chan struct{}, ctxErr chan<- error) fnTool {
	return fnTool{tool.Def{Name: "gate"}, func(ctx context.Context) (string, error) {
		select {
		case <-release:
			ctxErr <- nil
			return "released", nil
		case <-ctx.Done():
			ctxErr <- ctx.Err()
			return "", ctx.Err()
		}
	}}
}
