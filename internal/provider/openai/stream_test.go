package openai

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providertest"
)

func TestStreamText(t *testing.T) {
	evs, _, err := streamFixture(t, "text.sse", provider.Request{Model: "gpt-6.1-sol", Messages: []provider.Message{user("Hi")}})
	if err != nil {
		t.Fatal(err)
	}
	want := []provider.Event{
		&provider.Start{Model: "gpt-6.1-sol-2026-09-30"},
		&provider.TextDelta{Text: "Hello"},
		&provider.TextDelta{Text: " world"},
		&provider.Usage{Input: 20, Output: 7, CacheRead: 100},
		&provider.Stop{Reason: provider.StopEndTurn},
	}
	if !reflect.DeepEqual(evs, want) {
		t.Fatalf("events:\n got %#v\nwant %#v", evs, want)
	}
}

func TestStreamReasoning(t *testing.T) {
	evs, _, err := streamFixture(t, "reasoning.sse", provider.Request{Model: "gpt-6.1-sol"})
	if err != nil {
		t.Fatal(err)
	}
	var deltas string
	for _, d := range providertest.Only[*provider.ThinkingDelta](evs) {
		deltas += d.Text
	}
	if deltas != "Thinking.\n\nMore." {
		t.Errorf("thinking deltas = %q", deltas)
	}
	th := providertest.Only[*provider.Thinking](evs)
	if len(th) != 1 || th[0].Text != "Thinking.\n\nMore." {
		t.Fatalf("thinking = %#v", th)
	}
	if want := (reasoningRef{ID: "rs_1", Encrypted: "ENC-1"}).encode(); th[0].Signature != want {
		t.Errorf("signature = %q, want %q", th[0].Signature, want)
	}
}

// TestStreamHiddenReasoning: reasoning with no summary arrives as
// RedactedThinking; reasoning without encrypted content is not kept.
func TestStreamHiddenReasoning(t *testing.T) {
	evs, _, err := streamFixture(t, "reasoning_hidden.sse", provider.Request{Model: "gpt-6.1-sol"})
	if err != nil {
		t.Fatal(err)
	}
	red := providertest.Only[*provider.RedactedThinking](evs)
	if len(red) != 1 || red[0].Data != (reasoningRef{ID: "rs_2", Encrypted: "ENC-2"}).encode() {
		t.Fatalf("redacted = %#v", red)
	}
	if th := providertest.Only[*provider.Thinking](evs); len(th) != 0 {
		t.Fatalf("kept reasoning without encrypted content: %#v", th)
	}
}

func TestStreamToolCall(t *testing.T) {
	evs, _, err := streamFixture(t, "tool_call.sse", provider.Request{Model: "gpt-6.1-sol"})
	if err != nil {
		t.Fatal(err)
	}
	tools := providertest.Only[*provider.ToolUse](evs)
	if len(tools) != 2 ||
		tools[0].ID != "call_1" || tools[0].Name != "read" || string(tools[0].Input) != `{"path":"a.go"}` ||
		tools[1].ID != "call_2" || string(tools[1].Input) != "{}" {
		t.Fatalf("tool calls = %#v", tools)
	}
	if stop := providertest.Only[*provider.Stop](evs); len(stop) != 1 || stop[0].Reason != provider.StopToolUse {
		t.Fatalf("stop = %#v", stop)
	}
}

func TestStreamStopReasons(t *testing.T) {
	tests := map[string]provider.StopReason{
		"incomplete.sse": provider.StopMaxTokens,
		"refusal.sse":    provider.StopRefusal,
	}
	for fixture, want := range tests {
		evs, _, err := streamFixture(t, fixture, provider.Request{Model: "gpt-6.1-sol"})
		if err != nil {
			t.Fatalf("%s: %v", fixture, err)
		}
		stop := providertest.Only[*provider.Stop](evs)
		if len(stop) != 1 || stop[0].Reason != want || len(providertest.Only[*provider.Usage](evs)) != 1 {
			t.Errorf("%s: events %#v, want stop %s", fixture, evs, want)
		}
	}
}

// TestStreamErrors: an error inside a 200 stream ends it with its kind,
// after the events before it.
func TestStreamErrors(t *testing.T) {
	tests := map[string]provider.Kind{
		"failed.sse":       provider.KindRateLimited,
		"error_event.sse":  provider.KindContextTooLong,
		"error_object.sse": provider.KindOverloaded,
		"cut.sse":          provider.KindNetwork,
	}
	for fixture, want := range tests {
		evs, _, err := streamFixture(t, fixture, provider.Request{Model: "gpt-6.1-sol"})
		var pe *provider.Error
		if !errors.As(err, &pe) || pe.Kind != want {
			t.Errorf("%s: error %v, want %v", fixture, err, want)
			continue
		}
		if len(providertest.Only[*provider.TextDelta](evs)) != 1 {
			t.Errorf("%s: events before the error were lost: %#v", fixture, evs)
		}
	}
}

// TestStreamErrorRepeats: after an error, Next keeps returning it; it
// never reports the clean end of a reply that failed.
func TestStreamErrorRepeats(t *testing.T) {
	for _, fixture := range []string{"failed.sse", "error_event.sse"} {
		as := newAPIServer(t, fixture)
		s, err := newTest(as.URL).Stream(context.Background(), provider.Request{Model: "gpt-6.1-sol"})
		if err != nil {
			t.Fatal(err)
		}
		var first error
		for first == nil {
			_, first = s.Next()
		}
		for range 3 {
			if _, err := s.Next(); err != first {
				t.Errorf("%s: Next after %v = %v, want the same error", fixture, first, err)
			}
		}
		_ = s.Close()
	}
}

func TestStreamCancel(t *testing.T) {
	as := newAPIServer(t, "text.sse")
	ctx, cancel := context.WithCancel(context.Background())
	s, err := newTest(as.URL).Stream(ctx, provider.Request{Model: "gpt-6.1-sol"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if _, err := s.Next(); err != nil {
		t.Fatal(err)
	}
	cancel()
	if _, err := s.Next(); !errors.Is(err, context.Canceled) {
		t.Fatalf("Next after cancel = %v, want context.Canceled", err)
	}
}
