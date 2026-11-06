package clauderuntime

import (
	"context"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/shell"
)

// TestModelsFromInitialize: Snow reads Claude Code's models and their
// effort levels from the initialize reply, with no model request.
func TestModelsFromInitialize(t *testing.T) {
	p, err := NewProfile(t.TempDir(), "selected", Subscription)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	models, err := p.Models(ctx, testProgram(t, "conversation"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, m := range models {
		ids = append(ids, m.ID)
	}
	if !slices.Equal(ids, []string{"default", "opus", "haiku"}) {
		t.Fatalf("models = %q", ids)
	}
	if !slices.Contains(models[1].Efforts, "max") || len(models[2].Efforts) != 0 {
		t.Fatalf("efforts: opus %q, haiku %q", models[1].Efforts, models[2].Efforts)
	}
}

// TestModelFlag: Claude Code's default takes no --model flag; another
// model passes its alias.
func TestModelFlag(t *testing.T) {
	for model, want := range map[string]string{"claude-code/default": "", "claude-code/opus": "opus", "": ""} {
		if got := modelFlag(model); got != want {
			t.Errorf("modelFlag(%q) = %q, want %q", model, got, want)
		}
	}
}

// testProgram is the stream helper as a Claude Code program: the "--" keeps
// the test binary from reading Claude's flags.
func testProgram(t *testing.T, mode string) shell.Profile {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SNOW_CLAUDE_STREAM_TEST", mode)
	return shell.Profile{Path: exe, Args: []string{"-test.run=^TestStreamHelper$", "--"}}
}
