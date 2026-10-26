package api

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestModelListJSON: empty arrays travel as [], never null, so a client can
// range over them without a nil check.
func TestModelListJSON(t *testing.T) {
	b, err := json.Marshal(ModelList{Providers: []ProviderModels{{Name: "anthropic", Connected: true}}})
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if strings.Contains(got, "null") || !strings.Contains(got, `"models":[]`) {
		t.Fatalf("ModelList JSON = %s", got)
	}
	b, err = json.Marshal(ModelInfo{ID: "m"})
	if err != nil {
		t.Fatal(err)
	}
	if got := string(b); !strings.Contains(got, `"efforts":[]`) {
		t.Fatalf("ModelInfo JSON = %s", got)
	}
}

// TestEffortSupported: only the levels the model lists are supported.
func TestEffortSupported(t *testing.T) {
	m := ModelInfo{Efforts: []string{"low", "high"}}
	for _, tt := range []struct {
		level string
		want  bool
	}{{"low", true}, {"high", true}, {"medium", false}, {"max", false}} {
		if got := m.EffortSupported(tt.level); got != tt.want {
			t.Errorf("EffortSupported(%q) = %v, want %v", tt.level, got, tt.want)
		}
	}
}

// TestModelMessageRoundTrip: the new messages travel by their pinned types.
func TestModelMessageRoundTrip(t *testing.T) {
	for _, m := range []Message{
		&ModelListRequest{Force: true},
		&ModelList{Providers: []ProviderModels{{Name: "openai", Title: "OpenAI", Connected: true,
			Models: []ModelInfo{{ID: "m", DisplayName: "M", ContextWindow: 1000, Efforts: []string{"high"}}}}}},
	} {
		raw, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		got := newMessage(m.Type())
		if got == nil {
			t.Fatalf("%T: no message for type %d", m, m.Type())
		}
		if err := json.Unmarshal(raw, got); err != nil {
			t.Fatalf("%T: %v", m, err)
		}
		if a, b := string(raw), mustJSON(t, got); a != b {
			t.Errorf("%T round trip:\n%s\n%s", m, a, b)
		}
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
