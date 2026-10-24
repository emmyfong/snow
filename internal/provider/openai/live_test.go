package openai

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/credential"
	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providertest"
)

// liveEnv turns on TestLive, the manual check against the real API. It
// costs a fraction of a cent: one short reply from GPT-6 Luna.
const liveEnv = "SNOW_LIVE_OPENAI"

// TestLive checks the key, lists models, and streams one reply from the
// real OpenAI API. The key comes from OPENAI_API_KEY or Snow's saved key
// and is never printed.
func TestLive(t *testing.T) {
	if os.Getenv(liveEnv) == "" {
		t.Skip("set " + liveEnv + "=1 to run against the real API")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	p := New(credential.New(filepath.Join(home, ".snow"), nil))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := p.CheckKey(ctx); err != nil {
		t.Fatalf("CheckKey: %v", err)
	}
	models, err := p.Models(ctx)
	if err != nil || len(models) == 0 {
		t.Fatalf("Models: %d, %v", len(models), err)
	}
	for _, m := range models {
		t.Logf("model %s", m.ID)
	}

	const model = "gpt-6-luna"
	s, err := p.Stream(ctx, provider.Request{
		Model: model, Effort: provider.EffortLow, MaxTokens: 2000,
		Messages: []provider.Message{{Role: provider.RoleUser, Content: []provider.Block{&provider.Text{Text: "Reply with one short sentence."}}}},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	evs, err := providertest.Collect(t, s)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	var text string
	for _, d := range providertest.Only[*provider.TextDelta](evs) {
		text += d.Text
	}
	start, stop, usage := providertest.Only[*provider.Start](evs), providertest.Only[*provider.Stop](evs), providertest.Only[*provider.Usage](evs)
	if len(start) != 1 || len(stop) != 1 || len(usage) != 1 || text == "" {
		t.Fatalf("incomplete reply: %#v", evs)
	}
	cost, _ := provider.Cost(provider.ModelName{Provider: Name, Model: model}, *usage[0])
	t.Logf("model %s said %q; stop %s; usage %+v; est. $%.6f", start[0].Model, text, stop[0].Reason, *usage[0], cost)
}
