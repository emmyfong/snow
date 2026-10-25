package main

import (
	"log/slog"
	"slices"
	"testing"

	"github.com/emmyfong/snow/internal/config"
)

// TestChatOptions: chats start with the configured model, or the default
// of the first provider with a key; key variables are hidden from
// commands.
func TestChatOptions(t *testing.T) {
	authTest(t)
	log := slog.New(slog.DiscardHandler)
	cfg := config.Defaults()
	o, err := chatOptions(cfg, log)
	if err != nil {
		t.Fatal(err)
	}
	if m, _ := o.Model(); m != "anthropic/claude-sonnet-5-5" {
		t.Fatalf("no keys: %q", m)
	}
	if !slices.Contains(o.HideEnv, "ANTHROPIC_API_KEY") || !slices.Contains(o.HideEnv, "OPENAI_API_KEY") {
		t.Fatalf("hidden %v", o.HideEnv)
	}
	if _, ok := o.Providers.Get("openai"); !ok || o.Tools == nil || o.Store == nil {
		t.Fatalf("options %+v", o)
	}
	t.Setenv("OPENAI_API_KEY", goodKey)
	if o, err = chatOptions(cfg, log); err != nil {
		t.Fatal(err)
	}
	if m, _ := o.Model(); m != "openai/gpt-6.1-sol" {
		t.Fatalf("openai key: %q", m)
	}
	cfg.Chat = config.Chat{Model: "anthropic/claude-opus-5-5", Effort: "high"}
	if o, err = chatOptions(cfg, log); err != nil {
		t.Fatal(err)
	}
	if m, e := o.Model(); m != "anthropic/claude-opus-5-5" || e != "high" {
		t.Fatalf("configured: %q %q", m, e)
	}
}
