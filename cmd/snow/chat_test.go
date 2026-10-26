package main

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/emmyfong/snow/internal/config"
	"github.com/emmyfong/snow/internal/server"
)

// TestChatOptions: chats start with the configured model, or with none so
// the client asks; key variables are hidden from commands; the picker
// learns which providers have a key and can save a new default.
func TestChatOptions(t *testing.T) {
	dir := authTest(t)
	cfg := config.Defaults()
	o, err := chatOptions(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if o.DefaultModel != "" {
		t.Fatalf("no saved default: %q, want empty", o.DefaultModel)
	}
	if !slices.Contains(o.HideEnv, "ANTHROPIC_API_KEY") || !slices.Contains(o.HideEnv, "OPENAI_API_KEY") {
		t.Fatalf("hidden %v", o.HideEnv)
	}
	if _, ok := o.Providers.Get("openai"); !ok || o.Tools == nil || o.Store == nil || o.Auth == nil {
		t.Fatalf("options %+v", o)
	}
	connected := func(o *server.ChatOptions) map[string]bool {
		out := map[string]bool{}
		for _, p := range o.ProviderStatuses() {
			out[p.Name] = p.Connected
		}
		return out
	}
	if got := connected(o); got["anthropic"] || got["openai"] {
		t.Fatalf("no keys: %v", got)
	}
	t.Setenv("OPENAI_API_KEY", goodKey)
	if o, err = chatOptions(cfg); err != nil {
		t.Fatal(err)
	}
	if o.DefaultModel != "" {
		t.Fatalf("a key alone is no saved default: %q", o.DefaultModel)
	}
	if got := connected(o); !got["openai"] || got["anthropic"] {
		t.Fatalf("openai key: %v", got)
	}
	cfg.Chat = config.Chat{Model: "anthropic/claude-opus-5-5", Effort: "high"}
	if o, err = chatOptions(cfg); err != nil {
		t.Fatal(err)
	}
	if o.DefaultModel != "anthropic/claude-opus-5-5" || o.DefaultEffort != "high" {
		t.Fatalf("configured: %q %q", o.DefaultModel, o.DefaultEffort)
	}
	if err := o.SaveDefault("openai/gpt-6.1-sol", ""); err != nil {
		t.Fatalf("SaveDefault: %v", err)
	}
	saved, err := config.Load(filepath.Join(dir, "config.toml"))
	if err != nil || saved.Chat.Model != "openai/gpt-6.1-sol" || saved.Chat.Effort != "" {
		t.Fatalf("saved config: %+v, %v", saved.Chat, err)
	}
}
