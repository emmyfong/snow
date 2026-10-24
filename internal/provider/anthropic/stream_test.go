package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/emmyfong/snow/internal/provider"
	"github.com/emmyfong/snow/internal/provider/providertest"
)

func TestStreamText(t *testing.T) {
	evs, body, err := streamFixture(t, "text.sse", provider.Request{
		Model: "claude-opus-5-5", Effort: provider.EffortHigh, System: "Be brief.",
		Messages: []provider.Message{{Role: provider.RoleUser, Content: []provider.Block{&provider.Text{Text: "Hi"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var text string
	for _, d := range providertest.Only[*provider.TextDelta](evs) {
		text += d.Text
	}
	if text != "Hello, world" {
		t.Fatalf("text = %q", text)
	}
	if _, ok := evs[0].(*provider.Start); !ok {
		t.Fatalf("first event %T, want *Start", evs[0])
	}
	if s := evs[len(evs)-1].(*provider.Stop); s.Reason != provider.StopEndTurn {
		t.Fatalf("stop = %v", s.Reason)
	}
	// The request carries what Snow asked for.
	if body["model"] != "claude-opus-5-5" || body["stream"] != true {
		t.Fatalf("request model/stream = %v/%v", body["model"], body["stream"])
	}
	if oc, _ := body["output_config"].(map[string]any); oc["effort"] != "high" {
		t.Fatalf("output_config = %v", body["output_config"])
	}
	if th, _ := body["thinking"].(map[string]any); th["type"] != "adaptive" || th["display"] != "summarized" {
		t.Fatalf("thinking = %v", body["thinking"])
	}
}

func TestStreamThinking(t *testing.T) {
	evs, _, err := streamFixture(t, "thinking.sse", provider.Request{Model: "claude-opus-5-5"})
	if err != nil {
		t.Fatal(err)
	}
	var deltas string
	for _, d := range providertest.Only[*provider.ThinkingDelta](evs) {
		deltas += d.Text
	}
	blocks := providertest.Only[*provider.Thinking](evs)
	want := &provider.Thinking{Text: "The user greets me. I reply.", Signature: "sig-abc"}
	if deltas != want.Text || len(blocks) != 1 || !reflect.DeepEqual(blocks[0], want) {
		t.Fatalf("thinking deltas %q, blocks %v", deltas, blocks)
	}
}

func TestStreamToolUse(t *testing.T) {
	evs, _, err := streamFixture(t, "tool_use.sse", provider.Request{
		Model: "claude-opus-5-5",
		Tools: []provider.ToolDef{{Name: "read", Description: "Read a file", Schema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"],"additionalProperties":false}`)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	calls := providertest.Only[*provider.ToolUse](evs)
	if len(calls) != 1 || calls[0].ID != "toolu_01" || calls[0].Name != "read" {
		t.Fatalf("tool calls = %v", calls)
	}
	var input map[string]string
	if err := json.Unmarshal(calls[0].Input, &input); err != nil || input["path"] != "main.go" {
		t.Fatalf("input %s: %v", calls[0].Input, err)
	}
	if s := evs[len(evs)-1].(*provider.Stop); s.Reason != provider.StopToolUse {
		t.Fatalf("stop = %v", s.Reason)
	}
}

// TestToolSchemaSent: the tool's JSON Schema reaches the API whole.
func TestToolSchemaSent(t *testing.T) {
	_, body, _ := streamFixture(t, "text.sse", provider.Request{
		Model: "m",
		Tools: []provider.ToolDef{{Name: "read", Schema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"],"additionalProperties":false}`)}},
	})
	tools, _ := body["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("tools = %v", body["tools"])
	}
	schema := tools[0].(map[string]any)["input_schema"].(map[string]any)
	if schema["type"] != "object" || schema["additionalProperties"] != false || fmt.Sprint(schema["required"]) != "[path]" || schema["properties"] == nil {
		t.Fatalf("input_schema = %v", schema)
	}
}

// TestMessagesSent: every block kind goes back to the API in its own shape.
func TestMessagesSent(t *testing.T) {
	_, body, _ := streamFixture(t, "text.sse", provider.Request{
		Model: "m",
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: []provider.Block{&provider.Text{Text: "read main.go"}}},
			{Role: provider.RoleAssistant, Content: []provider.Block{
				&provider.Thinking{Text: "need the file", Signature: "sig"},
				&provider.RedactedThinking{Data: "opaque"},
				&provider.ToolUse{ID: "toolu_01", Name: "read", Input: json.RawMessage(`{"path":"main.go"}`)},
			}},
			{Role: provider.RoleUser, Content: []provider.Block{&provider.ToolResult{ToolUseID: "toolu_01", Content: "package main", IsError: false}}},
		},
	})
	msgs, _ := body["messages"].([]any)
	if len(msgs) != 3 {
		t.Fatalf("messages = %v", body["messages"])
	}
	types := func(m any) (out []string) {
		for _, c := range m.(map[string]any)["content"].([]any) {
			out = append(out, c.(map[string]any)["type"].(string))
		}
		return out
	}
	if got := fmt.Sprint(types(msgs[0]), types(msgs[1]), types(msgs[2])); got != "[text] [thinking redacted_thinking tool_use] [tool_result]" {
		t.Fatalf("block types = %s", got)
	}
}

func TestStreamUsage(t *testing.T) {
	evs, _, err := streamFixture(t, "usage.sse", provider.Request{Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	u := providertest.Only[*provider.Usage](evs)
	want := provider.Usage{Input: 3, Output: 7, CacheRead: 2000, CacheWrite: 500}
	if len(u) == 0 || *u[len(u)-1] != want {
		t.Fatalf("last usage = %v, want %v", u, want)
	}
}

// TestResponseModel: the stream reports the model the API says it used,
// even when it differs from the one asked for.
func TestResponseModel(t *testing.T) {
	evs, _, err := streamFixture(t, "model.sse", provider.Request{Model: "claude-opus-5-5"})
	if err != nil {
		t.Fatal(err)
	}
	if s := providertest.Only[*provider.Start](evs); len(s) != 1 || s[0].Model != "claude-opus-5-5-20261001" {
		t.Fatalf("start = %v", s)
	}
}

// TestStreamCutOff: a stream that ends without message_stop is a network
// error, so the turn can show the reply as cut off.
func TestStreamCutOff(t *testing.T) {
	evs, _, err := streamFixture(t, "cut.sse", provider.Request{Model: "m"})
	var pe *provider.Error
	if !errors.As(err, &pe) || pe.Kind != provider.KindNetwork {
		t.Fatalf("error = %v, want KindNetwork", err)
	}
	if len(providertest.Only[*provider.TextDelta](evs)) != 1 {
		t.Fatalf("the partial text was lost: %v", evs)
	}
}

// TestStreamCancel: canceling mid-stream ends it with the context error and
// leaves no goroutine behind.
func TestStreamCancel(t *testing.T) {
	sent := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			http.NotFound(w, r)
			return
		}
		// The server notices a client hang-up only once the body is read.
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		data, _ := os.ReadFile(filepath.Join("testdata", "cut.sse"))
		_, _ = w.Write(data)
		w.(http.Flusher).Flush()
		close(sent)
		<-r.Context().Done()
	}))
	defer srv.Close()
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
	before := runtime.NumGoroutine()

	ctx, cancel := context.WithCancel(context.Background())
	s, err := New(providertest.Key(testKey), WithBaseURL(srv.URL), WithHTTPClient(client)).Stream(ctx, provider.Request{Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Next(); err != nil {
		t.Fatal(err)
	}
	<-sent
	cancel()
	if _, err := s.Next(); !errors.Is(err, context.Canceled) {
		t.Fatalf("Next after cancel = %v, want context.Canceled", err)
	}
	if err := s.Close(); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("Close = %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n := runtime.NumGoroutine(); n > before {
		buf := make([]byte, 1<<16)
		t.Fatalf("%d goroutines after cancel, %d before:\n%s", n, before, buf[:runtime.Stack(buf, true)])
	}
}

// TestStreamRedactedThinking: encrypted reasoning comes through whole, to
// be sent back unchanged.
func TestStreamRedactedThinking(t *testing.T) {
	evs, _, err := streamFixture(t, "redacted.sse", provider.Request{Model: "claude-opus-5-5"})
	if err != nil {
		t.Fatal(err)
	}
	got := providertest.Only[*provider.RedactedThinking](evs)
	if len(got) != 1 || got[0].Data != "EncryptedOpaqueData==" {
		t.Fatalf("redacted thinking = %v", got)
	}
}

// TestNextAfterClose: after Close, Next ends the stream at once, as the
// fake does.
func TestNextAfterClose(t *testing.T) {
	srv := sseServer(t, "text.sse", nil)
	s, err := newTest(srv.URL).Stream(context.Background(), provider.Request{Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Next(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if ev, err := s.Next(); !errors.Is(err, io.EOF) {
			t.Fatalf("Next after Close = %v, %v; want io.EOF", ev, err)
		}
	}
}

// TestStopWithoutReason: a reply whose message_delta never came still ends
// with a reason.
func TestStopWithoutReason(t *testing.T) {
	evs, _, err := streamFixture(t, "nostop.sse", provider.Request{Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	if s, ok := evs[len(evs)-1].(*provider.Stop); !ok || s.Reason != provider.StopOther {
		t.Fatalf("last event = %#v, want Stop{StopOther}", evs[len(evs)-1])
	}
}
