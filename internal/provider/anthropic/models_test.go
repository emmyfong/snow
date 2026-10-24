package anthropic

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providertest"
)

func TestModels(t *testing.T) {
	got, err := newTest(newModelServer(t, testModels, "text.sse").URL).Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []provider.Model{
		{ID: "claude-opus-5-5", DisplayName: "claude-opus-5-5", ContextWindow: 1000000},
		{ID: "claude-haiku-4-5-20251001", DisplayName: "claude-haiku-4-5-20251001", ContextWindow: 1000000},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Models = %v, want %v", got, want)
	}
}

func TestCheckKey(t *testing.T) {
	srv := newModelServer(t, testModels, "text.sse")
	if err := newTest(srv.URL).CheckKey(context.Background()); err != nil {
		t.Fatalf("good key: %v", err)
	}
	err := New(providertest.Key("sk-ant-wrong"), WithBaseURL(srv.URL)).CheckKey(context.Background())
	var pe *provider.Error
	if !errors.As(err, &pe) || pe.Kind != provider.KindBadKey {
		t.Fatalf("bad key: %v, want KindBadKey", err)
	}
}

// TestThinkingOnlyWhenSupported: thinking and effort go only to a model
// that takes them, and the model list is read once.
func TestThinkingOnlyWhenSupported(t *testing.T) {
	tests := []struct {
		name         string
		models       []model
		model        string
		effort       provider.Effort
		wantThinking bool
		wantEffort   string
	}{
		{"current model", testModels, "claude-opus-5-5", provider.EffortHigh, true, "high"},
		{"effort level the model lacks", testModels, "claude-opus-5-5", provider.EffortXHigh, true, ""},
		{"older model by alias", testModels, "claude-haiku-4-5", provider.EffortHigh, false, ""},
		{"older model by snapshot ID", testModels, "claude-haiku-4-5-20251001", provider.EffortLow, false, ""},
		{"model not in the list", testModels, "claude-next", provider.EffortMax, true, "max"},
		{"list without capabilities", []model{{id: "claude-opus-5-5", lifecycle: "active", noCaps: true}}, "claude-opus-5-5", provider.EffortHigh, true, "high"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newModelServer(t, tt.models, "text.sse")
			p := newTest(srv.URL)
			for range 2 {
				s, err := p.Stream(context.Background(), provider.Request{Model: tt.model, Effort: tt.effort})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := providertest.Collect(t, s); err != nil {
					t.Fatal(err)
				}
			}
			if lists, _ := srv.counts(); lists != 1 {
				t.Errorf("model list read %d times, want 1", lists)
			}
			body := srv.body(0)
			th, _ := body["thinking"].(map[string]any)
			if got := th["type"] == "adaptive"; got != tt.wantThinking {
				t.Errorf("thinking = %v, want adaptive %v", body["thinking"], tt.wantThinking)
			}
			oc, _ := body["output_config"].(map[string]any)
			if got, _ := oc["effort"].(string); got != tt.wantEffort {
				t.Errorf("effort = %q, want %q", got, tt.wantEffort)
			}
		})
	}
}

// noKeys fails as a key store with nothing saved does.
type noKeys struct{}

var errNoKey = errors.New("nothing saved for anthropic")

func (noKeys) APIKey(string) (string, error) { return "", errNoKey }

// TestNoKey: with no key saved, no request is sent, and every call returns
// KindNoKey with the store's error as its cause.
func TestNoKey(t *testing.T) {
	srv := newModelServer(t, testModels, "text.sse")
	p := New(noKeys{}, WithBaseURL(srv.URL))
	_, streamErr := p.Stream(context.Background(), provider.Request{Model: "m"})
	_, modelsErr := p.Models(context.Background())
	for _, err := range []error{streamErr, modelsErr, p.CheckKey(context.Background())} {
		var pe *provider.Error
		if !errors.As(err, &pe) || pe.Kind != provider.KindNoKey || !errors.Is(err, errNoKey) {
			t.Errorf("error = %v, want KindNoKey wrapping the store's error", err)
		}
	}
	if lists, messages := srv.counts(); lists != 0 || messages != 0 {
		t.Fatalf("requests sent without a key: %d lists, %d messages", lists, messages)
	}
}

// TestCheckKeyForbidden: a 403 means the key is valid but lacks access to
// the model list, so the check passes.
func TestCheckKeyForbidden(t *testing.T) {
	srv := errServer(t, 403, "permission_error", "no access to models", nil)
	if err := newTest(srv.URL).CheckKey(context.Background()); err != nil {
		t.Fatalf("CheckKey on 403 = %v, want nil", err)
	}
}
