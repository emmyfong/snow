package openai

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/credential"
	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providertest"
)

// liveEnv turns on TestLive, the manual check against the real API. It
// costs a fraction of a cent: two short turns from GPT-6 Luna.
const liveEnv = "SNOW_LIVE_OPENAI"

// TestLive checks the key, lists models, and runs two turns against the
// real OpenAI API: the first calls a tool, the second sends back the
// reasoning, the call, and its result, as the agent loop will. The key
// comes from OPENAI_API_KEY or Snow's saved key and is never printed.
func TestLive(t *testing.T) {
	if os.Getenv(liveEnv) == "" {
		t.Skip("set " + liveEnv + "=1 to run against the real API")
	}
	dir, err := credential.DefaultDir()
	if err != nil {
		t.Fatal(err)
	}
	p := New(credential.New(dir, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := p.CheckKey(ctx); err != nil {
		t.Fatalf("CheckKey: %v", err)
	}
	models, err := p.Models(ctx)
	if err != nil || len(models) == 0 {
		t.Fatalf("Models: %d, %v", len(models), err)
	}
	t.Logf("%d models", len(models))

	const model = "gpt-6-luna"
	tools := []provider.ToolDef{{
		Name: "get_time", Description: "Returns the current time.",
		Schema: json.RawMessage(`{"type":"object","properties":{}}`),
	}}
	msgs := []provider.Message{{Role: provider.RoleUser, Content: []provider.Block{
		&provider.Text{Text: "Call get_time once, then tell me the time in one short sentence."},
	}}}
	turn := func(n int) []provider.Event {
		s, err := p.Stream(ctx, provider.Request{Model: model, Effort: provider.EffortLow, MaxTokens: 2000, Tools: tools, Messages: msgs})
		if err != nil {
			t.Fatalf("turn %d: Stream: %v", n, err)
		}
		evs, err := providertest.Collect(t, s)
		if err != nil {
			t.Fatalf("turn %d: Next: %v", n, err)
		}
		usage := providertest.Only[*provider.Usage](evs)
		stop := providertest.Only[*provider.Stop](evs)
		if len(usage) != 1 || len(stop) != 1 {
			t.Fatalf("turn %d: incomplete reply: %#v", n, evs)
		}
		cost, _ := provider.Cost(provider.ModelName{Provider: Name, Model: model}, *usage[0])
		t.Logf("turn %d: stop %s; usage %+v; est. $%.6f", n, stop[0].Reason, *usage[0], cost)
		return evs
	}

	evs := turn(1)
	calls := providertest.Only[*provider.ToolUse](evs)
	if len(calls) == 0 {
		t.Fatalf("turn 1 made no tool call: %#v", evs)
	}
	var reply []provider.Block
	var results []provider.Block
	for _, ev := range evs {
		switch e := ev.(type) {
		case *provider.Thinking:
			reply = append(reply, e)
		case *provider.RedactedThinking:
			reply = append(reply, e)
		case *provider.ToolUse:
			reply = append(reply, e)
			results = append(results, &provider.ToolResult{ToolUseID: e.ID, Content: "12:00 UTC"})
		}
	}
	msgs = append(msgs,
		provider.Message{Role: provider.RoleAssistant, Content: reply},
		provider.Message{Role: provider.RoleUser, Content: results})

	evs = turn(2)
	var text string
	for _, d := range providertest.Only[*provider.TextDelta](evs) {
		text += d.Text
	}
	if text == "" {
		t.Fatalf("turn 2 gave no text: %#v", evs)
	}
	t.Logf("turn 2 said %q", text)
}
