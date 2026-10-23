package anthropic

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/credential"
	"github.com/emmyfong/snow/internal/provider"
)

// liveEnv turns on TestLive, the manual check against the real API. It
// costs a few cents: one short reply from Haiku 5.5.
const liveEnv = "SNOW_LIVE_ANTHROPIC"

// TestLive checks the key, lists models, and streams one reply from the
// real Anthropic API. The key comes from ANTHROPIC_API_KEY or Snow's saved
// key and is never printed.
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
		t.Logf("model %s (%s), context %d", m.ID, m.DisplayName, m.ContextWindow)
	}

	s, err := p.Stream(ctx, provider.Request{
		Model: "claude-haiku-5-5", Effort: provider.EffortLow, MaxTokens: 2000,
		Messages: []provider.Message{{Role: provider.RoleUser, Content: []provider.Block{&provider.Text{Text: "Reply with one short sentence."}}}},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	defer func() { _ = s.Close() }()
	var text string
	var start *provider.Start
	var stop *provider.Stop
	var usage provider.Usage
	for {
		ev, err := s.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		switch e := ev.(type) {
		case *provider.Start:
			start = e
		case *provider.TextDelta:
			text += e.Text
		case *provider.Usage:
			usage = *e
		case *provider.Stop:
			stop = e
		}
	}
	if start == nil || stop == nil || text == "" {
		t.Fatalf("incomplete reply: start %v, stop %v, text %q", start, stop, text)
	}
	cost, _ := provider.Cost(provider.ModelName{Provider: Name, Model: "claude-haiku-5-5"}, usage)
	t.Logf("model %s said %q; stop %s; usage %+v; est. $%.6f", start.Model, text, stop.Reason, usage, cost)
}
