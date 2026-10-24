package openai

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providertest"
)

func TestModels(t *testing.T) {
	as := newAPIServer(t, "", "gpt-6.1-sol", "gpt-5.5-2026-04-23", "o3", "codex-mini-latest",
		"text-embedding-3-large", "gpt-image-2", "gpt-realtime", "gpt-4o-transcribe", "tts-1", "whisper-1",
		"dall-e-3", "omni-moderation-latest", "gpt-3.5-turbo-instruct", "davinci-002", "sora-2", "gpt-5-search-api")
	got, err := newTest(as.URL).Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, m := range got {
		if m.DisplayName != m.ID {
			t.Errorf("display name %q for %q", m.DisplayName, m.ID)
		}
		ids = append(ids, m.ID)
	}
	want := []string{"gpt-6.1-sol", "gpt-5.5-2026-04-23", "o3", "codex-mini-latest"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("models = %v, want %v", ids, want)
	}
	if w := provider.ContextWindow(provider.ModelName{Provider: Name, Model: got[1].ID}, got[1].ContextWindow); w != 1_050_000 {
		t.Errorf("context window of %s = %d, want the built-in 1050000", got[1].ID, w)
	}
}

func TestCheckKey(t *testing.T) {
	as := newAPIServer(t, "")
	if err := newTest(as.URL).CheckKey(context.Background()); err != nil {
		t.Fatalf("good key: %v", err)
	}
	err := New(providertest.Key("sk-wrong"), WithBaseURL(as.URL)).CheckKey(context.Background())
	var pe *provider.Error
	if !errors.As(err, &pe) || pe.Kind != provider.KindBadKey {
		t.Fatalf("bad key: %v, want KindBadKey", err)
	}
}

// TestCheckKeyForbidden: a 403 means the key is valid but may not list
// models, as a restricted project key without the Models scope. The key
// still works for replies, so the check passes.
func TestCheckKeyForbidden(t *testing.T) {
	srv := rawServer(t, 403, "application/json", errBody("", "invalid_request_error", "Missing scopes: api.model.read"), nil)
	if err := newTest(srv.URL).CheckKey(context.Background()); err != nil {
		t.Fatalf("CheckKey on 403 = %v, want nil", err)
	}
}

func TestCapsFor(t *testing.T) {
	tests := []struct {
		model     string
		reasoning bool
		takesMax  bool
	}{
		{"gpt-6.1-sol", true, true},
		{"gpt-5.5", true, false},
		{"gpt-5.5-2026-04-23", true, false},
		{"gpt-4.1", false, false},
		{"gpt-4o-2024-11-20", false, false},
		{"gpt-7", true, true},
		{"o3", true, true},
		{"codex-mini-latest", true, true},
		{"gpt-3.5-turbo", false, false},
		{"gpt-5-chat-latest", false, false},
		{"ft:gpt-4.1:org::abc", false, false},
	}
	for _, tt := range tests {
		c := capsFor(tt.model)
		if c.reasoning != tt.reasoning || (c.reasoning && c.takes(provider.EffortMax) != tt.takesMax) {
			t.Errorf("capsFor(%s) = %+v", tt.model, c)
		}
	}
}

// TestEveryKnownModelPriced: each model with built-in capabilities has a
// price and a context window, so the two lists stay in step.
func TestEveryKnownModelPriced(t *testing.T) {
	for id := range knownCaps {
		p, ok := provider.LookupPrice(provider.ModelName{Provider: Name, Model: id})
		if !ok || p.Input <= 0 || p.ContextWindow <= 0 {
			t.Errorf("%s has no price entry", id)
		}
	}
}
